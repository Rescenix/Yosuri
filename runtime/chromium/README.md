# Bundled Chromium runtime

The Windows installer may place a complete portable Chromium distribution in
this directory:

```text
runtime/chromium/
├── chrome.exe
├── chrome.dll
├── locales/
└── ...
```

ResceneAgent resolves the runtime relative to its own executable, so an
installed layout should look like:

```text
ResceneAgent.exe
runtime/chromium/chrome.exe
```

Do not copy only `chrome.exe`: Chromium requires its adjacent DLL, resource,
locale, and `.pak` files. The installer must also ship the Chromium license and
all required third-party notices. Browser binaries are intentionally not
committed to this repository; the release pipeline should obtain a pinned,
reviewed build, verify its checksum, and copy the full distribution here before
building the installer.

`CHROME_PATH` remains available as a development and diagnostics override.
