import sys
from pathlib import Path

# Ensure `src` package is importable when running pytest from the worker directory.
ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))
