const os = require('os');
const fs = require('fs');
const path = require('path');
const https = require('https');
const crypto = require('crypto');
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

const MAX_REDIRECTS = 5;

function request(url, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (!url.startsWith('https://')) {
      reject(new Error(`Refusing non-HTTPS URL: ${url}`));
      return;
    }
    https.get(url, (res) => {
      if ([301, 302, 303, 307, 308].includes(res.statusCode) && res.headers.location) {
        res.resume();
        if (redirects >= MAX_REDIRECTS) {
          reject(new Error('Too many redirects'));
          return;
        }
        resolve(request(new URL(res.headers.location, url).toString(), redirects + 1));
        return;
      }
      if (res.statusCode !== 200) {
        res.resume();
        reject(new Error(`Download failed: HTTP ${res.statusCode}`));
        return;
      }
      resolve(res);
    }).on('error', reject);
  });
}

async function download(url, dest) {
  const res = await request(url);
  await new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    res.pipe(file);
    file.on('finish', () => file.close(resolve));
    file.on('error', reject);
    res.on('error', reject);
  });
}

async function fetchText(url) {
  const res = await request(url);
  let body = '';
  res.setEncoding('utf8');
  for await (const chunk of res) body += chunk;
  return body;
}

function sha256File(file) {
  return crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
}

// checksums.txt lines look like "<sha256>  <filename>" (goreleaser format).
function expectedChecksum(checksums, filename) {
  for (const line of checksums.split('\n')) {
    const [hash, name] = line.trim().split(/\s+/);
    if (name === filename && /^[0-9a-f]{64}$/i.test(hash)) return hash.toLowerCase();
  }
  return null;
}

// Fails closed: a missing or mismatching checksum aborts the install.
// STACKLIT_SKIP_CHECKSUM=1 is an explicit escape hatch for mirrors.
async function verifyArchive(archivePath, url) {
  if (process.env.STACKLIT_SKIP_CHECKSUM === '1') {
    console.warn('Skipping checksum verification (STACKLIT_SKIP_CHECKSUM=1).');
    return;
  }
  const filename = url.split('/').pop();
  const sumsUrl = url.slice(0, url.lastIndexOf('/') + 1) + 'checksums.txt';
  const want = expectedChecksum(await fetchText(sumsUrl), filename);
  if (!want) throw new Error(`no checksum listed for ${filename}`);
  const got = sha256File(archivePath);
  if (got !== want) throw new Error(`checksum mismatch for ${filename}: expected ${want}, got ${got}`);
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
    await verifyArchive(archivePath, url);
  } catch (err) {
    fs.rmSync(archivePath, { force: true });
    console.error(`Checksum verification failed: ${err.message}`);
    console.error('Refusing to install an unverified binary. Install manually: go install github.com/glincker/stacklit/cmd/stacklit@latest');
    process.exitCode = 1;
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

module.exports = { expectedChecksum, sha256File };

if (require.main === module) install().catch((err) => {
  console.error('Installation failed:', err.message);
  console.error('You can install manually: go install github.com/glincker/stacklit/cmd/stacklit@latest');
  // Surface the failure to npm so postinstall doesn't appear to succeed when
  // the binary is actually missing or extraction broke.
  process.exitCode = 1;
});
