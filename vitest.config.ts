import { defineConfig } from 'vitest/config'

export default defineConfig({
    test: {
        testTimeout: 1200000,
        hookTimeout: 1200000,
        poolOptions: {
            vmForks: {
                // VM forks related options here
            },

        }
    }
})
