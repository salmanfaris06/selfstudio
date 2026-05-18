import path from 'node:path';
import type { StationConfig } from './stations.js';
import type { IngestEvent } from './ingest-log.js';

export type StationRuntimeState = {
  id: string;
  label: string;
  inputPath: string;
  status: 'starting' | 'running' | 'error';
  acceptedCount: number;
  lastFile: IngestEvent | null;
  lastError: string | null;
  lastIgnored: string | null;
  startedAt: string;
};

const stationStates = new Map<string, StationRuntimeState>();
const inProgressKeys = new Set<string>();
const acceptedKeys = new Set<string>();
const acceptedPaths = new Set<string>();

export function initializeStationState(configs: StationConfig[]): void {
  const now = new Date().toISOString();
  for (const config of configs) {
    stationStates.set(config.id, {
      id: config.id,
      label: config.label,
      inputPath: config.inputPath,
      status: 'starting',
      acceptedCount: 0,
      lastFile: null,
      lastError: null,
      lastIgnored: null,
      startedAt: now
    });
  }
}

export function hydrateStationStateFromEvents(events: IngestEvent[]): void {
  for (const event of events) {
    const state = stationStates.get(event.stationId);
    if (!state) {
      continue;
    }

    state.acceptedCount += 1;
    state.lastFile = event;
    markAcceptedDuplicateKey(buildDuplicateKey(event.sourcePath, event.fileSize, event.modifiedTime), event.sourcePath);
  }
}

export function getStationStates(): StationRuntimeState[] {
  return Array.from(stationStates.values());
}

export function getStationState(stationId: string): StationRuntimeState | undefined {
  return stationStates.get(stationId);
}

export function setStationRunning(stationId: string): void {
  const state = stationStates.get(stationId);
  if (state) {
    state.status = 'running';
    state.lastError = null;
  }
}

export function setStationError(stationId: string, error: string): void {
  const state = stationStates.get(stationId);
  if (state) {
    state.status = 'error';
    state.lastError = error;
  }
}

export function recordIgnoredFile(stationId: string, filePath: string): void {
  const state = stationStates.get(stationId);
  if (state) {
    state.lastIgnored = `${new Date().toISOString()} ${path.basename(filePath)}`;
  }
}

export function recordAcceptedFile(event: IngestEvent): void {
  const state = stationStates.get(event.stationId);
  if (state) {
    state.acceptedCount += 1;
    state.lastFile = event;
    state.lastError = null;
  }
}

export function normalizeFilePath(filePath: string): string {
  return path.resolve(filePath).toLowerCase();
}

export function buildDuplicateKey(filePath: string, fileSize: number, modifiedTime: string): string {
  return `${normalizeFilePath(filePath)}|${fileSize}|${modifiedTime}`;
}

export function beginFileProcessing(filePath: string): boolean {
  const key = normalizeFilePath(filePath);
  if (inProgressKeys.has(key) || acceptedPaths.has(key)) {
    return false;
  }
  inProgressKeys.add(key);
  return true;
}

export function finishFileProcessing(filePath: string): void {
  inProgressKeys.delete(normalizeFilePath(filePath));
}

export function hasAcceptedDuplicate(key: string, filePath?: string): boolean {
  return acceptedKeys.has(key) || (filePath ? acceptedPaths.has(normalizeFilePath(filePath)) : false);
}

export function markAcceptedDuplicateKey(key: string, filePath?: string): void {
  acceptedKeys.add(key);
  if (filePath) {
    acceptedPaths.add(normalizeFilePath(filePath));
  }
}
