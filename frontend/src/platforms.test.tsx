import type { Config } from '@bindings/ghost-chat/internal/config/models.js';

import { Platform } from '@bindings/ghost-chat/internal/chat/models.js';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { PLATFORMS, platformMeta } from './platforms';

describe('PLATFORMS', () => {
    it('lists the five chat platforms in home screen order', () => {
        expect(PLATFORMS.map((p) => p.id)).toEqual([
            Platform.PlatformTwitch,
            Platform.PlatformYouTube,
            Platform.PlatformYouTubeVertical,
            Platform.PlatformKick,
            Platform.PlatformTikTok,
        ]);
    });

    it('reads the saved input of each platform from its own config block', () => {
        const config = {
            twitch: { default_channel: 'tw' },
            youtube: { channel_id: '@landscape' },
            youtube_vertical: { channel_id: '@vertical' },
            kick: { default_channel: 'kk' },
            tiktok: { default_channel: 'tt' },
        } as Config;

        expect(PLATFORMS.map((p) => p.savedInput(config))).toEqual(['tw', '@landscape', '@vertical', 'kk', 'tt']);
    });

    it('returns an empty saved input before the config has loaded', () => {
        expect(PLATFORMS.map((p) => p.savedInput(null))).toEqual(['', '', '', '', '']);
    });

    it('gives every platform a distinct short label and a rendering icon', () => {
        const shorts = PLATFORMS.map((p) => p.short);

        expect(new Set(shorts).size).toBe(PLATFORMS.length);

        for (const p of PLATFORMS) {
            expect(renderToStaticMarkup(<p.Icon className="x" />)).toContain('<svg');
            expect(p.labelKey).toMatch(/^home\./);
            expect(p.placeholderKey).toMatch(/^home\.placeholder\./);
        }
    });

    it('only the stream-following platforms can wait for a stream', () => {
        expect(PLATFORMS.filter((p) => p.supportsWaiting).map((p) => p.id)).toEqual([
            Platform.PlatformYouTube,
            Platform.PlatformYouTubeVertical,
            Platform.PlatformTikTok,
        ]);
    });

    it('looks up a platform by id and returns undefined for unknown ids', () => {
        expect(platformMeta(Platform.PlatformTikTok)?.labelKey).toBe('home.tiktok');
        expect(platformMeta('nope' as Platform)).toBeUndefined();
    });
});
