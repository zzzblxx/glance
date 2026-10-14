"""Collect the exact installed distribution notices for a release build."""
from pathlib import Path
import importlib.metadata
import shutil
import subprocess

out = Path("docs/third-party")
out.mkdir(parents=True, exist_ok=True)
modules = subprocess.check_output(["go", "list", "-m", "-f", "{{.Dir}}", "all"], text=True).splitlines()[1:]
for directory in modules:
    root = Path(directory)
    for name in ("LICENSE", "COPYING", "NOTICE"):
        source = root / name
        if source.is_file():
            shutil.copyfile(source, out / (root.name + "-" + name + ".txt"))
for dist in importlib.metadata.distributions():
    for file in dist.files or ():
        if any(marker in file.name.lower() for marker in ("license", "copying", "notice")):
            source = Path(dist.locate_file(file))
            if source.is_file() and source.suffix not in (".py", ".pyc", ".so"):
                name = dist.metadata["Name"] + "-" + str(file).replace("/", "_")
                shutil.copyfile(source, out / name)
for lib in ("Seeed_GFX2", "ArduinoJson"):
    root = Path("firmware/e1003/.pio/libdeps/e1003") / lib
    for source in root.glob("LICENSE*"):
        if source.is_file():
            shutil.copyfile(source, out / (lib + "-" + source.name + ".txt"))
