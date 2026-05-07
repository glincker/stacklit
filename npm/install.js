const os = require('os');
const fs = require('fs');
const path = require('path');
const https = require('https');
const { execFileSync } = require('child_process');

// Read the version from package.json so the npm publish workflow's
// `npm version <tag>` bump is the single source of truth. Previously this was
// a hard-coded constant that drifted from package.json on every release,
// causing the postinstall to pull an outdated binary archive.
const VERSION = require('./package.json').version;
const REPO = 'glincker/stacklit';

const PLATFORM_MAP = {
  'darwin-arm64': 'stacklit_VERSION_darwin_arm64.tar.gz',
  'darwin-x64': 'stacklit_VERSION_darwin_amd64.tar.gz',
  'linux-arm64': 'stacklit_VERSION_linux_arm64.tar.gz',
  'linux-x64': 'stacklit_VERSION_linux_amd64.tar.gz',
  'win32-x64': 'stacklit_VERSION_windows_amd64.zip',
};

function getPlatformKey() {
  const platform = os.platform();
  const arch = os.arch();
  return `${platform}-${arch}`;
}

function getDownloadUrl() {
  const key = getPlatformKey();
  const filename = PLATFORM_MAP[key];
  if (!filename) {
    console.error(`Unsupported platform: ${key}`);
    console.error('Supported: ' + Object.keys(PLATFORM_MAP).join(', '));
    process.exit(1);
  }
  return `https://github.com/${REPO}/releases/download/v${VERSION}/${filename.replace('VERSION', VERSION)}`;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (url) => {
      https.get(url, (res) => {
        if (res.statusCode === 302 || res.statusCode === 301) {
          return follow(res.headers.location);
        }
        if (res.statusCode !== 200) {
          reject(new Error(`Download failed: HTTP ${res.statusCode}`));
          return;
        }
        const file = fs.createWriteStream(dest);
        res.pipe(file);
        file.on('finish', () => { file.close(); resolve(); });
      }).on('error', reject);
    };
    follow(url);
  });
}

async function install() {
  const binDir = path.join(__dirname, 'bin');
  const binPath = path.join(binDir, os.platform() === 'win32' ? 'stacklit.exe' : 'stacklit');

  // Skip if binary already exists
  if (fs.existsSync(binPath)) {
    return;
  }

  fs.mkdirSync(binDir, { recursive: true });

  const url = getDownloadUrl();
  const archivePath = path.join(__dirname, 'archive.tmp');

  console.log(`Downloading stacklit v${VERSION}...`);

  try {
    await download(url, archivePath);
  } catch (err) {
    console.error(`Failed to download: ${err.message}`);
    console.error('You can install manually: go install github.com/glincker/stacklit/cmd/stacklit@latest');
    // Don't fail the install — let the bin script show a helpful error
    return;
  }

  try {
    // Extract. execFileSync passes args directly to the binary, so no shell
    // quoting of paths is needed. On Windows we hand paths to PowerShell via
    // env vars so a single quote in __dirname (e.g. C:\Users\O'Connor\...)
    // cannot terminate the script string.
    if (url.endsWith('.zip')) {
      if (os.platform() === 'win32') {
        execFileSync(
          'powershell',
          [
            '-NoProfile',
            '-ExecutionPolicy', 'Bypass',
            '-Command',
            'Expand-Archive -Force -LiteralPath $env:STACKLIT_ARCHIVE -DestinationPath $env:STACKLIT_BINDIR',
          ],
          {
            stdio: 'inherit',
            env: { ...process.env, STACKLIT_ARCHIVE: archivePath, STACKLIT_BINDIR: binDir },
          }
        );
      } else {
        execFileSync('unzip', ['-o', archivePath, 'stacklit.exe', '-d', binDir], { stdio: 'inherit' });
      }
    } else {
      execFileSync('tar', ['-xzf', archivePath, '-C', binDir, 'stacklit'], { stdio: 'inherit' });
    }

    if (!fs.existsSync(binPath)) {
      throw new Error(`extraction completed but ${binPath} is missing`);
    }

    if (os.platform() !== 'win32') {
      fs.chmodSync(binPath, 0o755);
    }
  } finally {
    if (fs.existsSync(archivePath)) {
      try {
        fs.unlinkSync(archivePath);
      } catch (cleanupErr) {
        // best-effort cleanup; warn but don't mask the original failure
        console.warn(`Could not remove ${archivePath}: ${cleanupErr.message}`);
      }
    }
  }

  console.log('stacklit installed successfully.');
}

install().catch((err) => {
  console.error('Installation failed:', err.message);
  console.error('You can install manually: go install github.com/glincker/stacklit/cmd/stacklit@latest');
  // Surface the failure to npm so postinstall doesn't appear to succeed when
  // the binary is actually missing or extraction broke.
  process.exitCode = 1;
});
