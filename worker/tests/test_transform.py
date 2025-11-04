from src.etl import transform


def test_parse_city_country():
    components = [
        {"long_name": "Gotham", "types": ["locality"]},
        {"long_name": "USA", "types": ["country"]},
    ]
    city, country = transform.parse_city_country(components)
    assert city == "Gotham"
    assert country == "USA"

    city, country = transform.parse_city_country([])
    assert city is None and country is None


def test_extract_primary_type():
    assert transform._extract_primary_type(["point_of_interest", "restaurant"]) == "restaurant"
    assert transform._extract_primary_type([]) is None


def test_to_company_row_uses_fallbacks():
    result = {
        "name": "Acme",
        "formatted_address": "Main St",
        "formatted_phone_number": "123",
        "website": "https://example.com",
        "rating": 4.5,
        "user_ratings_total": 10,
        "types": ["store"],
        "geometry": {"location": {"lng": 10, "lat": 20}},
    }

    row = transform.to_company_row(result, fallback_city="Gotham", fallback_country="USA")

    assert row["company"] == "Acme"
    assert row["city"] == "Gotham"
    assert row["country"] == "USA"
    assert row["lng"] == 10
    assert row["type_business"] == "store"
