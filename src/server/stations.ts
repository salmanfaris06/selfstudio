import path from 'node:path';

export type StationConfig = {
  id: 'camera-1' | 'camera-2' | 'camera-3';
  label: string;
  inputPath: string;
};

const projectRoot = process.cwd();

export const stations: StationConfig[] = [
  {
    id: 'camera-1',
    label: 'Camera 1',
    inputPath: path.join(projectRoot, 'data', 'input', 'camera-1')
  },
  {
    id: 'camera-2',
    label: 'Camera 2',
    inputPath: path.join(projectRoot, 'data', 'input', 'camera-2')
  },
  {
    id: 'camera-3',
    label: 'Camera 3',
    inputPath: path.join(projectRoot, 'data', 'input', 'camera-3')
  }
];
