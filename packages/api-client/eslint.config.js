import base from '@heatseeker/config/eslint';
import { defineConfig, globalIgnores } from 'eslint/config';

export default defineConfig([globalIgnores(['src/schema.d.ts']), ...base]);
