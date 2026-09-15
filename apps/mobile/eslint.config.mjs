import base from '@heatseeker/config/eslint';
import { defineConfig, globalIgnores } from 'eslint/config';

export default defineConfig([globalIgnores(['.expo/**', 'expo-env.d.ts', 'android/**', 'ios/**']), ...base]);
