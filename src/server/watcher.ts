import fs from 'node:fs/promises';
import path from 'node:path';
import chokidar, { FSWatcher } from 'chokidar';
import type { StationConfig } from './stations.js';
import { appendIngestEvent } from './ingest-log.js';
import { isJpgPath, waitForStableJpg } from './stable-file.js';
import {
  beginFileProcessing,
  buildDuplicateKey,
  finishFileProcessing,
  hasAcceptedDuplicate,
  markAcceptedDuplicateKey,
  recordAcceptedFile,
  recordIgnoredFile,
  setStationError,
  setStationRunning
} from './state.js';

export async function startStationWatchers(stations: StationConfig[]): Promise<FSWatcher[]> {
  const watchers: FSWatcher[] = [];

  for (const station of stations) {
    try {
      const folderStat = await fs.stat(station.inputPath);
      if (!folderStat.isDirectory()) {
        throw new Error('Input path exists but is not a directory');
      }

      const watcher = chokidar.watch(station.inputPath, {
        ignoreInitial: true,
        persistent: true,
        depth: 0,
        awaitWriteFinish: false
      });

      watcher.on('ready', () => setStationRunning(station.id));
      watcher.on('add', (filePath) => void handleFileEvent(station, filePath));
      watcher.on('change', (filePath) => void handleFileEvent(station, filePath));
      watcher.on('unlinkDir', (removedPath) => {
        if (path.resolve(removedPath) === path.resolve(station.inputPath)) {
          setStationError(station.id, 'Input folder was removed');
        }
      });
      watcher.on('error', (error) => setStationError(station.id, String(error)));

      watchers.push(watcher);
    } catch (error) {
      setStationError(station.id, (error as Error).message);
    }
  }

  return watchers;
}

async function handleFileEvent(station: StationConfig, filePath: string): Promise<void> {
  if (!isJpgPath(filePath)) {
    recordIgnoredFile(station.id, filePath);
    return;
  }

  if (!beginFileProcessing(filePath)) {
    return;
  }

  try {
    const stable = await waitForStableJpg(filePath);
    const duplicateKey = buildDuplicateKey(filePath, stable.size, stable.mtimeIso);
    if (hasAcceptedDuplicate(duplicateKey, filePath)) {
      return;
    }

    const event = {
      timestamp: new Date().toISOString(),
      stationId: station.id,
      sourcePath: path.resolve(filePath),
      filename: path.basename(filePath),
      fileSize: stable.size,
      modifiedTime: stable.mtimeIso,
      status: 'accepted' as const
    };

    await appendIngestEvent(event);
    markAcceptedDuplicateKey(duplicateKey, filePath);
    recordAcceptedFile(event);
  } catch (error) {
    setStationError(station.id, `${path.basename(filePath)}: ${(error as Error).message}`);
  } finally {
    finishFileProcessing(filePath);
  }
}
