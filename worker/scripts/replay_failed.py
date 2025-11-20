import os, json, requests, time
FOLDER = "worker/data/failed"
URL = "http://localhost:8080/ingest/places"   # change to your ingestion URL if needed

if not os.path.isdir(FOLDER):
    print("No failed folder:", FOLDER)
    raise SystemExit(0)

for fname in sorted(os.listdir(FOLDER)):
    path = os.path.join(FOLDER, fname)
    try:
        with open(path, "r", encoding="utf-8") as fh:
            payload = json.load(fh)
        r = requests.post(URL, json=payload, timeout=10)
        r.raise_for_status()
        print("Replayed", fname, "=>", r.status_code)
        os.remove(path)
    except Exception as e:
        print("Failed to replay", fname, e)
        time.sleep(1)
