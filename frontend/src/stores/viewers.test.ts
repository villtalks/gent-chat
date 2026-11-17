import { Platform } from '@bindings/ghost-chat/internal/chat/models.js';
import { beforeEach, describe, expect, it } from 'vitest';

import { selectTotal, useViewersStore } from './viewers';

describe('viewers store', () => {
    beforeEach(() => {
        useViewersStore.setState({ counts: {} });
    });

    it('sums the counts of live platforms only', () => {
        const { apply } = useViewersStore.getState();

        apply({ platform: Platform.PlatformTwitch, count: 412, live: true });
        apply({ platform: Platform.PlatformYouTube, count: 530, live: true });
        apply({ platform: Platform.PlatformKick, count: 99, live: false });

        expect(selectTotal(useViewersStore.getState())).toBe(942);
    });

    it('drops a platform from the total when it stops being live', () => {
        const { apply } = useViewersStore.getState();

        apply({ platform: Platform.PlatformTwitch, count: 412, live: true });
        apply({ platform: Platform.PlatformTikTok, count: 100, live: true });
        apply({ platform: Platform.PlatformTwitch, count: 0, live: false });

        expect(selectTotal(useViewersStore.getState())).toBe(100);
        expect(useViewersStore.getState().counts[Platform.PlatformTwitch]).toEqual({ count: 0, live: false });
    });

    it('replaces the previous count of a platform instead of adding to it', () => {
        const { apply } = useViewersStore.getState();

        apply({ platform: Platform.PlatformTikTok, count: 100, live: true });
        apply({ platform: Platform.PlatformTikTok, count: 103, live: true });

        expect(selectTotal(useViewersStore.getState())).toBe(103);
    });

    it('seeds from a list and tolerates a null list', () => {
        const { seed } = useViewersStore.getState();

        seed(null);
        expect(selectTotal(useViewersStore.getState())).toBe(0);

        seed([
            { platform: Platform.PlatformTwitch, count: 5, live: true },
            { platform: Platform.PlatformKick, count: 7, live: true },
        ]);

        expect(selectTotal(useViewersStore.getState())).toBe(12);
    });

    it('ignores negative and non-numeric counts', () => {
        const { apply } = useViewersStore.getState();

        apply({ platform: Platform.PlatformTwitch, count: -5, live: true });
        apply({ platform: Platform.PlatformKick, count: Number.NaN, live: true });

        expect(selectTotal(useViewersStore.getState())).toBe(0);
    });
});
