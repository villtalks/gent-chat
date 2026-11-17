import { Platform } from '@bindings/ghost-chat/internal/chat/models.js';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { ViewerBar, formatCount } from './ViewerBar';

const counts = {
    [Platform.PlatformTwitch]: { count: 412, live: true },
    [Platform.PlatformYouTube]: { count: 530, live: true },
    [Platform.PlatformYouTubeVertical]: { count: 201, live: true },
    [Platform.PlatformKick]: { count: 38, live: true },
    [Platform.PlatformTikTok]: { count: 103, live: true },
};

const all = [
    Platform.PlatformTwitch,
    Platform.PlatformYouTube,
    Platform.PlatformYouTubeVertical,
    Platform.PlatformKick,
    Platform.PlatformTikTok,
];

function render(props: Partial<Parameters<typeof ViewerBar>[0]> = {}) {
    return renderToStaticMarkup(
        <ViewerBar
            platforms={all}
            counts={counts}
            showTotal
            showPlatforms
            {...props}
        />
    );
}

function chipValues(html: string): string[] {
    return [...html.matchAll(/data-platform="([a-z_]+)"[^>]*>.*?<span[^>]*>([^<]*)<\/span><\/span>/g)].map(
        (m) => `${m[1]}=${m[2]}`
    );
}

describe('ViewerBar', () => {
    it('renders the total with a thousands separator', () => {
        expect(render()).toContain('>1,284<');
    });

    it('renders one chip per platform in home screen order', () => {
        expect(chipValues(render())).toEqual([
            'twitch=412',
            'youtube=530',
            'youtube_vertical=201',
            'kick=38',
            'tiktok=103',
        ]);
    });

    it('orders chips by platform order regardless of the order passed in', () => {
        const html = render({ platforms: [Platform.PlatformTikTok, Platform.PlatformTwitch] });

        expect(chipValues(html)).toEqual(['twitch=412', 'tiktok=103']);
    });

    it('shows a dash for a platform that is not live and leaves it out of the total', () => {
        const html = render({
            platforms: [Platform.PlatformTwitch, Platform.PlatformKick],
            counts: {
                [Platform.PlatformTwitch]: { count: 412, live: true },
                [Platform.PlatformKick]: { count: 0, live: false },
            },
        });

        expect(chipValues(html)).toEqual(['twitch=412', 'kick=–']);
        expect(html).toContain('>412<');
    });

    it('shows a dash for a platform with no count yet', () => {
        const html = render({ platforms: [Platform.PlatformTikTok], counts: {} });

        expect(chipValues(html)).toEqual(['tiktok=–']);
        expect(html).toContain('>0<');
    });

    it('totals only the platforms it was given', () => {
        const html = render({ platforms: [Platform.PlatformTwitch, Platform.PlatformKick] });

        expect(html).toContain('>450<');
    });

    it('renders only the total when per-platform counts are switched off', () => {
        const html = render({ showPlatforms: false });

        expect(html).toContain('>1,284<');
        expect(chipValues(html)).toEqual([]);
    });

    it('renders nothing when the viewer count is switched off', () => {
        expect(render({ showTotal: false })).toBe('');
    });

    it('renders nothing when no platform is connected', () => {
        expect(render({ platforms: [] })).toBe('');
    });

    it('never paints a background of its own', () => {
        expect(render()).not.toMatch(/background/);
    });
});

describe('formatCount', () => {
    it('groups thousands', () => {
        expect(formatCount(0)).toBe('0');
        expect(formatCount(999)).toBe('999');
        expect(formatCount(1284)).toBe('1,284');
        expect(formatCount(1234567)).toBe('1,234,567');
    });
});
