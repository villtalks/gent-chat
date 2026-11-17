import type { YouTubeConfig } from '@bindings/ghost-chat/internal/config/models.js';

import type { DeepPartial } from '@/types/utils';

import { ResolveYouTubeVideo } from '@bindings/ghost-chat/app.js';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useConfigStore } from '@/stores/config';

import { BlacklistField } from './fields/BlacklistField';
import { ChannelField } from './fields/ChannelField';
import { ConnectOnStartupField } from './fields/ConnectOnStartupField';
import { FadeControls } from './fields/FadeControls';

type YouTubeConfigKey = 'youtube' | 'youtube_vertical';

export function YouTubeSettings({ configKey = 'youtube' }: { configKey?: YouTubeConfigKey }) {
    const { t } = useTranslation();
    const yt = useConfigStore((s) => s.config?.[configKey]);
    const vertical = configKey === 'youtube_vertical';
    const update = useConfigStore((s) => s.update);

    const [channelText, setChannelText] = useState(yt?.channel_id ?? '');
    const [resolving, setResolving] = useState(false);
    const [resolveError, setResolveError] = useState<string | null>(null);
    const [resolvedUrl, setResolvedUrl] = useState<string | null>(null);

    const set = (partial: DeepPartial<YouTubeConfig>) => update({ [configKey]: partial });

    const handleAutoDetect = async () => {
        if (!channelText) {
            return;
        }

        setResolving(true);
        setResolveError(null);
        setResolvedUrl(null);

        try {
            const url = await ResolveYouTubeVideo(channelText);

            setResolvedUrl(url);
            void set({ channel_id: channelText, video_url: url });
        } catch (err) {
            setResolveError(String(err));
        } finally {
            setResolving(false);
        }
    };

    return (
        <>
            <ChannelField
                initialValue={yt?.channel_id ?? ''}
                labelKey="settings.youtube.channel"
                placeholderKey={
                    vertical ? 'settings.youtube_vertical.channel_placeholder' : 'settings.youtube.channel_placeholder'
                }
                onSave={(v) => {
                    setResolveError(null);
                    setResolvedUrl(null);
                    void set({ channel_id: v });
                }}
                onTextChange={(v) => {
                    setChannelText(v);
                    setResolvedUrl(null);
                    setResolveError(null);
                }}
                footer={
                    <>
                        {resolvedUrl && <span className="field-success">{resolvedUrl}</span>}
                        {resolveError && <span className="field-error">{resolveError}</span>}
                    </>
                }
            >
                {!vertical && (
                    <button
                        className="btn btn-ghost"
                        onClick={handleAutoDetect}
                        disabled={!channelText || resolving}
                        style={{ flexShrink: 0 }}
                    >
                        {resolving ? t('settings.youtube.detecting') : t('settings.youtube.detect')}
                    </button>
                )}
            </ChannelField>

            {vertical && <span className="field-hint">{t('settings.youtube_vertical.hint')}</span>}

            <ConnectOnStartupField
                checked={yt?.connect_on_startup ?? false}
                channel={yt?.channel_id ?? ''}
                onChange={(v) => set({ connect_on_startup: v })}
            />

            <FadeControls
                fade={yt?.fade ?? false}
                fadeTimeout={yt?.fade_timeout ?? 30}
                onFadeChange={(v) => set({ fade: v })}
                onFadeTimeoutChange={(v) => set({ fade_timeout: v })}
            />

            <BlacklistField
                initialValue={yt?.user_blacklist ?? []}
                onSave={(v) => set({ user_blacklist: v })}
            />
        </>
    );
}
