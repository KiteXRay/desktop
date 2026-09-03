export function formatBytes(bytes: number, decimals: number = 0): string {
  if (!bytes || bytes <= 0) return '0 B';

  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];

  let i = Math.floor(Math.log(bytes) / Math.log(k));
  if (i >= sizes.length) i = sizes.length - 1;
  const val = bytes / Math.pow(k, i);

  if (decimals === 0) {
    let rounded = Math.round(val);
    if (rounded >= 1024 && i < sizes.length - 1) {
      rounded = 1;
      i++;
    }
    return `${rounded} ${sizes[i]}`;
  }

  return `${val.toFixed(decimals)} ${sizes[i]}`;
}

export function formatSpeed(kbPerSec: number): string {
  if (!kbPerSec || kbPerSec <= 0) return '0 KB/s';

  if (kbPerSec >= 1024) {
    return `${(kbPerSec / 1024).toFixed(2)} MB/s`;
  }
  return `${kbPerSec.toFixed(1)} KB/s`;
}
