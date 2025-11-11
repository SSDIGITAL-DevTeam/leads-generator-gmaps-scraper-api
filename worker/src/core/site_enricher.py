"""Website enrichment utilities for extracting public contact data."""

from __future__ import annotations

import importlib
import logging
import random
import re
import time
from collections import defaultdict
from typing import Any, Dict, List, Optional, Set, Tuple
from urllib import robotparser
from urllib.parse import urljoin, urlparse, urlunparse

import requests
from bs4 import BeautifulSoup

try:
    from playwright.sync_api import TimeoutError as PlaywrightTimeoutError, sync_playwright
except ImportError:  # pragma: no cover - optional dependency
    sync_playwright = None
    PlaywrightTimeoutError = Exception

from src.core.config import Settings, get_settings

logger = logging.getLogger(__name__)

USER_AGENT = "LeadsGeneratorBot/1.0 (+https://leads-generator.app/contact)"
REQUEST_TIMEOUT = 10
REQUEST_DELAY_RANGE = (1.0, 2.0)
MAX_PAGES_PER_DOMAIN = 3
SOCIAL_HOSTS = {
    "linkedin": ("linkedin.com",),
    "facebook": ("facebook.com", "fb.com"),
    "instagram": ("instagram.com", "instagr.am"),
    "youtube": ("youtube.com", "youtu.be"),
    "tiktok": ("tiktok.com",),
}
CONTACT_PAGE_CANDIDATES = (
    "/contact",
    "/contact-us",
    "/contactus",
    "/kontak",
    "/hubungi",
    "/about",
    "/tentang",
)
CONTACT_KEYWORDS = ("contact", "kontak", "hubungi")
ABOUT_KEYWORDS = ("about", "tentang")

EMAIL_REGEX = re.compile(r"[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}", re.IGNORECASE)
PHONE_CANDIDATE_REGEX = re.compile(r"\+?\d[\d\s().\-]{6,}")


class PlaywrightRenderer:
    """Thin wrapper around Playwright to render JavaScript-heavy pages."""

    def __init__(self, timeout_ms: int = 15000) -> None:
        if sync_playwright is None:
            raise RuntimeError("playwright is not installed")
        self._playwright = None
        self._browser = None
        self._timeout_ms = timeout_ms

    def _ensure_browser(self) -> None:
        if self._playwright is None:
            self._playwright = sync_playwright().start()
            self._browser = self._playwright.chromium.launch(headless=True)

    def render(self, url: str) -> Tuple[str, str]:
        self._ensure_browser()
        page = self._browser.new_page()
        try:
            page.goto(url, wait_until="networkidle", timeout=self._timeout_ms)
            content = page.content()
            final_url = page.url
            return final_url, content
        finally:
            page.close()

    def close(self) -> None:
        if self._browser is not None:
            self._browser.close()
            self._browser = None
        if self._playwright is not None:
            self._playwright.stop()
            self._playwright = None

COUNTRY_CODE_MAP = {
    "US": "1",
    "CA": "1",
    "GB": "44",
    "UK": "44",
    "AU": "61",
    "NZ": "64",
    "SG": "65",
    "MY": "60",
    "ID": "62",
    "PH": "63",
    "VN": "84",
    "TH": "66",
    "IN": "91",
    "JP": "81",
    "KR": "82",
    "CN": "86",
    "DE": "49",
    "FR": "33",
    "ES": "34",
    "IT": "39",
    "NL": "31",
    "BE": "32",
    "DK": "45",
    "SE": "46",
    "NO": "47",
    "FI": "358",
    "IE": "353",
    "BR": "55",
    "MX": "52",
    "AR": "54",
    "CL": "56",
    "ZA": "27",
    "NG": "234",
    "KE": "254",
    "AE": "971",
}

_PHONENUMBERS_MODULE: Optional[Any] = None
_PHONENUMBERS_LOADED = False


def _get_phonenumbers() -> Optional[Any]:
    global _PHONENUMBERS_MODULE, _PHONENUMBERS_LOADED
    if not _PHONENUMBERS_LOADED:
        try:
            _PHONENUMBERS_MODULE = importlib.import_module("phonenumbers")
        except ImportError:
            logger.warning(
                "phonenumbers package is not installed; falling back to limited phone normalization"
            )
            _PHONENUMBERS_MODULE = None
        finally:
            _PHONENUMBERS_LOADED = True
    return _PHONENUMBERS_MODULE  # type: ignore[return-value]


def sanitize_website(raw_url: str) -> Optional[str]:
    """Normalise raw website strings into absolute https URLs."""

    if not raw_url:
        return None

    url = raw_url.strip()
    if not url:
        return None

    parsed = urlparse(url, scheme="https")
    if not parsed.netloc:
        parsed = urlparse(f"https://{url}")

    if not parsed.netloc:
        return None

    normalized_path = parsed.path or "/"
    if not normalized_path.startswith("/"):
        normalized_path = f"/{normalized_path}"

    normalized = parsed._replace(path=normalized_path, fragment="", query="")
    return urlunparse(normalized)


def fetch_url(session: requests.Session, url: str, *, timeout: int = REQUEST_TIMEOUT) -> Optional[Tuple[str, BeautifulSoup]]:
    """Fetch a URL and return the final URL + soup when it is HTML content."""

    try:
        response = session.get(
            url,
            timeout=timeout,
            allow_redirects=True,
        )
        content_type = response.headers.get("Content-Type", "").lower()
        if "text/html" not in content_type:
            logger.debug("Skipping non-HTML content at %s (content-type=%s)", url, content_type)
            return None
        return response.url, BeautifulSoup(response.text, "html.parser")
    except requests.RequestException as exc:  # noqa: BLE001
        logger.warning("Failed to fetch %s: %s", url, exc)
        return None


def extract_emails(text: str) -> List[str]:
    """Return unique emails discovered in a text blob."""

    candidates = {match.group(0).lower() for match in EMAIL_REGEX.finditer(text or "")}
    return sorted(candidates)


def extract_phones(text: str, default_region: Optional[str] = None) -> List[str]:
    """Return E.164 phone strings parsed from text when possible."""

    if not text:
        return []

    normalized: Set[str] = set()
    phone_lib = _get_phonenumbers()
    for raw in PHONE_CANDIDATE_REGEX.findall(text):
        candidate = _normalize_phone(raw.strip(), default_region, phone_lib)
        if candidate:
            normalized.add(candidate)

    return sorted(normalized)


def _normalize_phone(raw: str, default_region: Optional[str], phone_lib: Optional[Any]) -> Optional[str]:
    if phone_lib:
        try:
            parsed = phone_lib.parse(raw, default_region)
        except phone_lib.NumberParseException:  # type: ignore[attr-defined]
            return None

        if not phone_lib.is_possible_number(parsed):
            return None

        return phone_lib.format_number(parsed, phone_lib.PhoneNumberFormat.E164)

    digits_only = re.sub(r"\D", "", raw)
    stripped = raw.strip()

    if stripped.startswith("+"):
        candidate = f"+{digits_only}"
        return _validate_e164(candidate)

    if stripped.startswith("00") and len(digits_only) > 2:
        candidate = f"+{digits_only[2:]}"
        return _validate_e164(candidate)

    country_code = COUNTRY_CODE_MAP.get((default_region or "").upper())
    if country_code:
        candidate = f"+{country_code}{digits_only}"
        return _validate_e164(candidate)

    return None


def _validate_e164(value: str) -> Optional[str]:
    if value.startswith("+") and 8 <= len(value) <= 16:
        return value
    return None


def extract_social_links(soup: BeautifulSoup, base_url: str) -> Dict[str, List[str]]:
    """Collect platform-specific social URLs present in anchor tags."""

    results: Dict[str, Set[str]] = {platform: set() for platform in SOCIAL_HOSTS}
    for anchor in soup.find_all("a", href=True):
        href = anchor["href"].strip()
        if not href:
            continue

        absolute = urljoin(base_url, href)
        parsed = urlparse(absolute)
        if parsed.scheme != "https" or not parsed.netloc:
            continue

        host = parsed.netloc.lower()
        for platform, allowed_hosts in SOCIAL_HOSTS.items():
            if any(allowed in host for allowed in allowed_hosts):
                normalized = urlunparse((parsed.scheme, parsed.netloc, parsed.path.rstrip("/"), "", "", ""))
                results[platform].add(normalized)

    return {platform: sorted(links) for platform, links in results.items() if links}


def extract_address(soup: BeautifulSoup) -> Optional[str]:
    """Attempt to extract a postal address-like snippet from a page."""

    address_selectors = [
        "[itemprop='address']",
        "address",
        ".address",
        "#address",
        "[class*='addr']",
        "[id*='addr']",
        "[class*='alamat']",
        "[id*='alamat']",
        "[class*='lokasi']",
        "[id*='lokasi']",
    ]

    for selector in address_selectors:
        for node in soup.select(selector):
            text = " ".join(node.stripped_strings)
            if len(text) >= 10:
                return text[:500]

    # Fallback: look for address keywords inside paragraphs/list items.
    keywords = ("street", "st.", "road", "rd", "jalan", "jl", "avenue", "ave", "alamat")
    for tag in soup.find_all(["p", "li", "span"]):
        text = tag.get_text(" ", strip=True)
        lowered = text.lower()
        if any(keyword in lowered for keyword in keywords) and len(text) > 15:
            return text[:500]

    return None


def _needs_js_render(soup: BeautifulSoup) -> bool:
    body_text = soup.get_text(" ", strip=True)
    if len(body_text) > 200:
        return False

    if soup.find(attrs={"data-page": True}):
        return True

    root = soup.find(id=re.compile("(app|root)", re.IGNORECASE))
    if root and not root.get_text(strip=True):
        return True

    return False


def _summarize_text(text: str, *, max_length: int = 320) -> Optional[str]:
    cleaned = re.sub(r"\s+", " ", text or "").strip()
    if not cleaned:
        return None
    if len(cleaned) <= max_length:
        return cleaned

    truncated = cleaned[: max_length + 1]
    last_space = truncated.rfind(" ")
    if last_space > 0:
        truncated = truncated[:last_space]
    return f"{truncated.rstrip('. ')}..."


class SiteEnricher:
    """Crawl a limited set of pages for a domain to extract contact information."""

    def __init__(
        self,
        website: str,
        *,
        settings: Optional[Settings] = None,
        session: Optional[requests.Session] = None,
        max_pages: int = MAX_PAGES_PER_DOMAIN,
    ) -> None:
        sanitized = sanitize_website(website)
        if not sanitized:
            raise ValueError("A valid website URL is required for enrichment")

        self.root_url = sanitized
        parsed = urlparse(self.root_url)
        self.domain = parsed.netloc.lower().lstrip("www.")
        self.base_origin = f"{parsed.scheme}://{parsed.netloc}"

        self.settings = settings or get_settings()
        self.max_pages = max_pages
        self.session = session or requests.Session()
        self.session.headers.setdefault("User-Agent", USER_AGENT)
        self.session.headers.setdefault("Accept", "text/html,application/xhtml+xml")
        self.session.headers.setdefault("Accept-Language", "en-US,en;q=0.9")

        self._robots = self._load_robot_rules(parsed)
        self.use_js_renderer = bool(getattr(self.settings, "enrich_use_js_renderer", False))
        self._js_renderer: Optional[PlaywrightRenderer] = None
        if self.use_js_renderer and sync_playwright is None:
            logger.warning("Playwright is unavailable; disabling JS renderer")
            self.use_js_renderer = False

    @staticmethod
    def _load_robot_rules(parsed_url) -> Optional[robotparser.RobotFileParser]:
        robots_url = urlunparse((parsed_url.scheme, parsed_url.netloc, "/robots.txt", "", "", ""))
        parser_obj = robotparser.RobotFileParser()
        parser_obj.set_url(robots_url)
        try:
            parser_obj.read()
            return parser_obj
        except Exception as exc:  # noqa: BLE001
            logger.debug("Unable to read robots.txt from %s: %s", robots_url, exc)
            return None

    def _is_same_domain(self, url: str) -> bool:
        parsed = urlparse(url)
        if not parsed.netloc:
            return True  # relative URLs inherit domain
        return parsed.netloc.lower().lstrip("www.") == self.domain

    def _is_allowed_by_robots(self, url: str) -> bool:
        if not self._robots:
            return True
        parsed = urlparse(url)
        path = parsed.path or "/"
        allowed = self._robots.can_fetch(USER_AGENT, path)
        if not allowed:
            logger.info("Robots.txt disallows %s", url)
        return allowed

    def _build_candidate_urls(self, base_url: str, soup: BeautifulSoup) -> List[str]:
        discovered: List[str] = []
        anchors = soup.find_all("a", href=True)
        for anchor in anchors:
            href = anchor["href"].strip()
            if not href:
                continue
            absolute = urljoin(base_url, href)
            if not self._is_same_domain(absolute):
                continue
            parsed = urlparse(absolute)
            normalized_path = parsed.path.lower()
            if any(candidate in normalized_path for candidate in CONTACT_PAGE_CANDIDATES):
                discovered.append(urlunparse((parsed.scheme, parsed.netloc, parsed.path, "", "", "")))

        # Ensure deterministic order and unique entries
        unique: List[str] = []
        seen: Set[str] = set()
        for candidate in discovered:
            if candidate not in seen:
                seen.add(candidate)
                unique.append(candidate)
        return unique

    def _find_contact_form(self, page_url: str, soup: BeautifulSoup) -> Optional[str]:
        for form in soup.find_all("form"):
            descriptor = " ".join(
                filter(
                    None,
                    [
                        form.get("id", ""),
                        form.get("name", ""),
                        " ".join(form.get("class", [])) if isinstance(form.get("class"), list) else form.get("class", ""),
                    ],
                )
            ).lower()
            action = (form.get("action") or "").strip()
            action_lower = action.lower()
            if any(keyword in descriptor for keyword in CONTACT_KEYWORDS) or any(
                keyword in action_lower for keyword in CONTACT_KEYWORDS
            ):
                if action:
                    return urljoin(page_url, action)
                return page_url

        for anchor in soup.find_all("a", href=True):
            text = anchor.get_text(" ", strip=True).lower()
            if any(keyword in text for keyword in CONTACT_KEYWORDS):
                candidate = urljoin(page_url, anchor["href"])
                if self._is_same_domain(candidate):
                    return candidate

        return None

    def _looks_like_about(self, url: str) -> bool:
        path = urlparse(url).path.lower()
        return any(keyword in path for keyword in ABOUT_KEYWORDS)

    def _extract_mailto_links(self, soup: BeautifulSoup) -> Set[str]:
        emails: Set[str] = set()
        for anchor in soup.find_all("a", href=True):
            href = anchor["href"].strip()
            if href.lower().startswith("mailto:"):
                value = href.split(":", 1)[1]
                email = value.split("?")[0].strip().lower()
                if email:
                    emails.add(email)
        return emails

    def _extract_tel_links(self, soup: BeautifulSoup) -> Set[str]:
        numbers: Set[str] = set()
        for anchor in soup.find_all("a", href=True):
            href = anchor["href"].strip()
            if href.lower().startswith("tel:"):
                payload = href.split(":", 1)[1]
                parsed_numbers = extract_phones(payload, self.settings.default_phone_region)
                numbers.update(parsed_numbers)
        return numbers

    def _get_js_renderer(self) -> Optional[PlaywrightRenderer]:
        if not self.use_js_renderer:
            return None
        if not self._js_renderer:
            try:
                self._js_renderer = PlaywrightRenderer(timeout_ms=REQUEST_TIMEOUT * 1000)
            except RuntimeError as exc:
                logger.warning("Unable to initialise Playwright renderer: %s", exc)
                self.use_js_renderer = False
                return None
        return self._js_renderer

    def _fetch_with_js(self, url: str) -> Optional[Tuple[str, BeautifulSoup]]:
        renderer = self._get_js_renderer()
        if not renderer:
            return None
        try:
            final_url, html = renderer.render(url)
            return final_url, BeautifulSoup(html, "html.parser")
        except PlaywrightTimeoutError:
            logger.warning("Playwright timed out fetching %s", url)
        except Exception as exc:  # noqa: BLE001
            logger.warning("Playwright failed for %s: %s", url, exc)
        return None

    def enrich(self) -> Dict[str, Any]:
        visit_queue: List[str] = [self.root_url]
        visited: Set[str] = set()
        aggregated_emails: Set[str] = set()
        aggregated_phones: Set[str] = set()
        aggregated_socials: Dict[str, Set[str]] = defaultdict(set)
        contact_form_url: Optional[str] = None
        address: Optional[str] = None
        about_summary: Optional[str] = None

        if not self._is_allowed_by_robots(self.root_url):
            logger.info("Robots disallows root path for %s; skipping enrichment", self.domain)
            return {
                "website": self.root_url,
                "pages_crawled": 0,
                "emails": [],
                "phones": [],
                "socials": {},
                "address": None,
                "contact_form_url": None,
                "about_summary": None,
            }

        delay_needed = False

        while visit_queue and len(visited) < self.max_pages:
            current_url = visit_queue.pop(0)
            if not self._is_same_domain(current_url):
                continue

            if not self._is_allowed_by_robots(current_url):
                continue

            if delay_needed:
                time.sleep(random.uniform(*REQUEST_DELAY_RANGE))

            fetched = fetch_url(self.session, current_url)
            delay_needed = True
            if not fetched:
                if self.use_js_renderer:
                    fetched = self._fetch_with_js(current_url)
                if not fetched:
                    continue

            final_url, soup = fetched

            if self.use_js_renderer and _needs_js_render(soup):
                js_fetched = self._fetch_with_js(final_url)
                if js_fetched:
                    final_url, soup = js_fetched

            if final_url in visited:
                continue

            visited.add(final_url)

            text = soup.get_text(" ", strip=True)
            aggregated_emails.update(extract_emails(text))
            aggregated_emails.update(self._extract_mailto_links(soup))
            aggregated_phones.update(extract_phones(text, self.settings.default_phone_region))
            aggregated_phones.update(self._extract_tel_links(soup))

            social_links = extract_social_links(soup, final_url)
            for platform, links in social_links.items():
                aggregated_socials[platform].update(links)

            if not address:
                address = extract_address(soup)

            if not contact_form_url:
                contact_form_url = self._find_contact_form(final_url, soup)

            if self._looks_like_about(final_url) and not about_summary:
                about_summary = _summarize_text(text)
            elif not about_summary:
                about_summary = _summarize_text(self._extract_about_section(soup)) or about_summary

            for candidate in self._build_candidate_urls(final_url, soup):
                if len(visited) + len(visit_queue) >= self.max_pages:
                    break
                if candidate not in visited and candidate not in visit_queue:
                    visit_queue.append(candidate)

        return {
            "website": self.root_url,
            "pages_crawled": len(visited),
            "emails": sorted(aggregated_emails),
            "phones": sorted(aggregated_phones),
            "socials": {platform: sorted(links) for platform, links in aggregated_socials.items() if links},
            "address": address,
            "contact_form_url": contact_form_url,
            "about_summary": about_summary,
        }

    def _extract_about_section(self, soup: BeautifulSoup) -> str:
        sections = soup.find_all(["section", "div"])
        for section in sections:
            descriptor = " ".join(
                filter(
                    None,
                    [
                        section.get("id", ""),
                        " ".join(section.get("class", [])) if isinstance(section.get("class"), list) else section.get("class", ""),
                    ],
                )
            ).lower()
            if any(keyword in descriptor for keyword in ABOUT_KEYWORDS):
                return section.get_text(" ", strip=True)
        return ""

    def close(self) -> None:
        self.session.close()
        if self._js_renderer:
            self._js_renderer.close()
            self._js_renderer = None

    def __enter__(self) -> "SiteEnricher":
        return self

    def __exit__(self, exc_type, exc, tb) -> None:  # noqa: D401
        self.close()


def post_enrich_result(company_id: str, data: Dict[str, Any], settings: Optional[Settings] = None) -> None:
    """POST enrichment payloads back to the Golang API callback."""

    settings = settings or get_settings()
    if not settings.enrich_callback_url:
        logger.warning("ENRICH_CALLBACK_URL missing; skipping callback for %s", company_id)
        return

    payload = {
        "company_id": company_id,
        "emails": data.get("emails", []),
        "phones": data.get("phones", []),
        "socials": data.get("socials", {}),
        "address": data.get("address"),
        "contact_form_url": data.get("contact_form_url"),
        "about_summary": data.get("about_summary"),
        "website": data.get("website"),
        "pages_crawled": data.get("pages_crawled"),
    }

    try:
        response = requests.post(
            settings.enrich_callback_url.rstrip("/") + "/enrich-result",
            json=payload,
            timeout=REQUEST_TIMEOUT,
            headers={"User-Agent": USER_AGENT},
        )
        response.raise_for_status()
    except requests.RequestException as exc:  # noqa: BLE001
        logger.exception("Failed to POST enrichment result for %s: %s", company_id, exc)
