import { Platform } from '@bindings/ghost-chat/internal/chat/models.js';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { platformMeta } from '@/platforms';

import { PlatformCard, platformButtonState } from './PlatformCard';

const tiktok = platformMeta(Platform.PlatformTikTok)!;

function render(overrides: Partial<Parameters<typeof PlatformCard>[0]> = {}) {
    return renderToStaticMarkup(
        <PlatformCard
            meta={tiktok}
            label="TikTok"
            placeholder="@name or live link"
            value=""
            buttonText="Connect"
            state="idle"
            busy={false}
            onChange={() => {}}
            onToggle={() => {}}
            {...overrides}
        />
    );
}

describe('PlatformCard', () => {
    it('renders the label, placeholder and icon', () => {
        const html = render();

        expect(html).toContain('TikTok');
        expect(html).toContain('placeholder="@name or live link"');
        expect(html).toContain('<svg');
    });

    it('disables the button while the input is empty', () => {
        expect(render({ value: '' })).toMatch(/<button[^>]*disabled/);
        expect(render({ value: '@somename' })).not.toMatch(/<button[^>]*disabled/);
    });

    it('disables the button while another connect is in flight', () => {
        expect(render({ value: '@somename', busy: true })).toMatch(/<button[^>]*disabled/);
    });

    it('uses the primary style when idle and the danger style when active', () => {
        expect(render({ value: 'x', state: 'idle' })).toContain('btn-primary');
        expect(render({ value: 'x', state: 'connected' })).toContain('btn-danger');
        expect(render({ value: 'x', state: 'waiting' })).toContain('btn-danger');
    });

    it('keeps an active card clickable even with an empty input so it can be disconnected', () => {
        expect(render({ value: '', state: 'connected' })).not.toMatch(/<button[^>]*disabled/);
    });
});

describe('platformButtonState', () => {
    it('reports waiting before connected, and idle otherwise', () => {
        expect(platformButtonState(true, true)).toBe('waiting');
        expect(platformButtonState(true, false)).toBe('connected');
        expect(platformButtonState(false, true)).toBe('waiting');
        expect(platformButtonState(false, false)).toBe('idle');
        expect(platformButtonState(undefined, undefined)).toBe('idle');
    });
});
