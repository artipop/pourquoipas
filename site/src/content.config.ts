import {docsSchema} from '@astrojs/starlight/schema'
import {glob} from 'astro/loaders'
import {defineCollection} from 'astro:content'

import {guideDir, guideId} from './guide/links.mjs'

// Starlight's own loader reads src/content/docs/; the guide is read from
// docs/guide/ instead, where the repository keeps it.
export const collections = {
    docs: defineCollection({
        loader: glob({
            base: guideDir,
            pattern: '**/[^_]*.{md,mdx}',
            generateId: ({entry}) => guideId(entry),
        }),
        schema: docsSchema(),
    }),
}
