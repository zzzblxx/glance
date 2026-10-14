"""Stage shared assets under a stable project-relative linker symbol."""
from pathlib import Path
import shutil

Import("env")
project = Path(env["PROJECT_DIR"])
source = project.parent.parent / "assets/fonts/unifont16.bin"
target = project / "generated/unifont16.bin"
target.parent.mkdir(parents=True, exist_ok=True)
if not target.exists() or target.read_bytes() != source.read_bytes():
    shutil.copyfile(source, target)
