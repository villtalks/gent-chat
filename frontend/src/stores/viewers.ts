import type { Platform } from '@bindings/ghost-chat/internal/chat/models.js';

import { create } from 'zustand';

export interface ViewerCount {
    count: number;
    live: boolean;
}

export interface ViewerUpdate extends ViewerCount {
    platform: Platform;
}

export type ViewerCounts = Partial<Record<Platform, ViewerCount>>;

interface ViewersState {
    counts: ViewerCounts;
    apply: (update: ViewerUpdate) => void;
    seed: (updates: ViewerUpdate[] | null | undefined) => void;
}

function clean(update: ViewerUpdate): ViewerCount {
    const count = Number.isFinite(update.count) && update.count > 0 ? Math.floor(update.count) : 0;

    return { count: update.live ? count : 0, live: Boolean(update.live) };
}

export const useViewersStore = create<ViewersState>((set) => ({
    counts: {},

    apply: (update) => set((s) => ({ counts: { ...s.counts, [update.platform]: clean(update) } })),

    seed: (updates) => {
        if (!updates) {
            return;
        }

        set((s) => {
            const counts = { ...s.counts };

            for (const update of updates) {
                counts[update.platform] = clean(update);
            }

            return { counts };
        });
    },
}));

export function totalOf(counts: ViewerCounts, platforms?: Platform[]): number {
    const ids = platforms ?? (Object.keys(counts) as Platform[]);

    return ids.reduce((sum, id) => {
        const entry = counts[id];

        return entry?.live ? sum + entry.count : sum;
    }, 0);
}

export function selectTotal(state: Pick<ViewersState, 'counts'>): number {
    return totalOf(state.counts);
}
