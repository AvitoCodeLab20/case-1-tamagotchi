import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import svgr from 'vite-plugin-svgr';
import path from 'node:path';
import { readFileSync } from 'node:fs';

const { version } = JSON.parse(
    readFileSync(new URL('../package.json', import.meta.url), 'utf8')
) as {
    version: string;
};

export default defineConfig({
    plugins: [react(), svgr()],
    define: {
        __APP_VERSION__: JSON.stringify(version),
    },
    resolve: {
        alias: {
            '@app': path.resolve(__dirname, './src/app'),
            '@pages': path.resolve(__dirname, './src/pages'),
            '@widgets': path.resolve(__dirname, './src/widgets'),
            '@features': path.resolve(__dirname, './src/features'),
            '@shared': path.resolve(__dirname, './src/shared'),
            '@assets': path.resolve(__dirname, './src/assets'),
            '@entities': path.resolve(__dirname, './src/entities'),
        },
    },
});
