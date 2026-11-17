import type { TikTokConfig } from '@bindings/ghost-chat/internal/config/models.js';

import type { DeepPartial } from '@/types/utils';

import { useConfigStore } from '@/stores/config';
import { validateTikTokChannel } from '@/utils/validate';

import { BlacklistField } from './fields/BlacklistField';
import { ChannelField } from './fields/ChannelField';
import { ConnectOnStartupField } from './fields/ConnectOnStartupField';
import { FadeControls } from './fields/FadeControls';

export function TikTokSettings() {
    const tiktok = useConfigStore((s) => s.config?.tiktok);
    const update = useConfigStore((s) => s.update);

    const set = (partial: DeepPartial<TikTokConfig>) => update({ tiktok: partial });

    return (
        <>
            <ChannelField
                initialValue={tiktok?.default_channel ?? ''}
                labelKey="settings.platform.default_channel"
                placeholderKey="settings.tiktok.default_channel_placeholder"
                onSave={(v) => set({ default_channel: v })}
                normalize={(v) => v.toLowerCase().replace(/^@/, '')}
                validator={validateTikTokChannel}
            />

            <ConnectOnStartupField
                checked={tiktok?.connect_on_startup ?? false}
                channel={tiktok?.default_channel ?? ''}
                onChange={(v) => set({ connect_on_startup: v })}
            />

            <FadeControls
                fade={tiktok?.fade ?? false}
                fadeTimeout={tiktok?.fade_timeout ?? 30}
                onFadeChange={(v) => set({ fade: v })}
                onFadeTimeoutChange={(v) => set({ fade_timeout: v })}
            />

            <BlacklistField
                initialValue={tiktok?.user_blacklist ?? []}
                onSave={(v) => set({ user_blacklist: v })}
            />
        </>
    );
}
