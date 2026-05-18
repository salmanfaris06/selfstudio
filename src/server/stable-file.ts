import { stat } from 'node:fs/promises';

export type StableFileResult = {
  size: number;
  mtimeMs: number;
  mtimeIso: string;
};

type StableFileOptions = {
  checks?: number;
  intervalMs?: number;
  maxWaitMs?: number;
};

const DEFAULT_CHECKS = 3;
const DEFAULT_INTERVAL_MS = 500;
const DEFAULT_MAX_WAIT_MS = 30_000;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export function isJpgPath(filePath: string): boolean {
  return /\.(jpe?g)$/i.test(filePath);
}

export async function waitForStableJpg(
  filePath: string,
  options: StableFileOptions = {}
): Promise<StableFileResult> {
  if (!isJpgPath(filePath)) {
    throw new Error('Unsupported file type; only .jpg and .jpeg are accepted');
  }

  const requiredStableChecks = options.checks ?? DEFAULT_CHECKS;
  const intervalMs = options.intervalMs ?? DEFAULT_INTERVAL_MS;
  const maxWaitMs = options.maxWaitMs ?? DEFAULT_MAX_WAIT_MS;

  if (!Number.isInteger(requiredStableChecks) || requiredStableChecks < 1) {
    throw new Error('Stable file checks must be a positive integer');
  }
  if (!Number.isFinite(intervalMs) || intervalMs < 1) {
    throw new Error('Stable file interval must be a positive number');
  }
  if (!Number.isFinite(maxWaitMs) || maxWaitMs < intervalMs) {
    throw new Error('Stable file max wait must be greater than or equal to interval');
  }

  const startedAt = Date.now();
  let stableChecks = 0;
  let previous: { size: number; mtimeMs: number } | null = null;

  while (stableChecks < requiredStableChecks) {
    if (Date.now() - startedAt > maxWaitMs) {
      throw new Error(`Timed out waiting for stable file after ${maxWaitMs}ms`);
    }

    const currentStat = await stat(filePath);
    const current = {
      size: currentStat.size,
      mtimeMs: currentStat.mtimeMs
    };

    if (
      previous &&
      current.size === previous.size &&
      current.mtimeMs === previous.mtimeMs &&
      current.size > 0
    ) {
      stableChecks += 1;
    } else {
      stableChecks = 0;
    }

    previous = current;

    if (stableChecks < requiredStableChecks) {
      await sleep(intervalMs);
    }
  }

  if (!previous) {
    throw new Error('Unable to determine stable file metadata');
  }

  const finalStat = await stat(filePath);
  if (finalStat.size !== previous.size || finalStat.mtimeMs !== previous.mtimeMs) {
    throw new Error('File changed after stability check; waiting for the next watcher event');
  }

  return {
    size: finalStat.size,
    mtimeMs: finalStat.mtimeMs,
    mtimeIso: finalStat.mtime.toISOString()
  };
}
