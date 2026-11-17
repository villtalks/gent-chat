import type { Platform } from '@bindings/ghost-chat/internal/chat/models.js';

import type { PlatformMeta } from '@/platforms';

import { Connect, Disconnect } from '@bindings/ghost-chat/app.js';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';

import { PLATFORMS } from '@/platforms';
import { useConfigStore } from '@/stores/config';
import { useConnectionStore } from '@/stores/connection';
import { isValidSourceURL } from '@/utils/customSource';

import styles from './Home.module.css';
import { PlatformCard, platformButtonState } from './PlatformCard';

export function Home() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const config = useConfigStore((s) => s.config);
    const update = useConfigStore((s) => s.update);

    const { connected, waiting, inputs, error, setInput, setError } = useConnectionStore();
    const [connecting, setConnecting] = useState<Platform | null>(null);
    const [sourceURL, setSourceURL] = useState(config?.custom_source?.url ?? '');

    const getInput = (meta: PlatformMeta): string => inputs[meta.id] || meta.savedInput(config);

    const anyActive = Object.values(connected).some(Boolean) || Object.values(waiting).some(Boolean);

    const handleOpenSource = () => {
        void update({ custom_source: { url: sourceURL } }, { silent: true }).then(() => navigate('/source'));
    };

    const handleToggle = async (meta: PlatformMeta) => {
        setError(null);

        if (connected[meta.id] || waiting[meta.id]) {
            void Disconnect(meta.id);

            return;
        }

        setConnecting(meta.id);

        try {
            await Connect(meta.id, getInput(meta).trim());
        } catch (err) {
            setError(String(err));
        } finally {
            setConnecting(null);
        }
    };

    const buttonText = (meta: PlatformMeta): string => {
        if (connecting === meta.id) {
            return t('home.connecting');
        }

        if (waiting[meta.id]) {
            return t('home.waiting');
        }

        return connected[meta.id] ? t('home.disconnect') : t('home.connect');
    };

    return (
        <div className={styles.home}>
            {error && <div className={styles.error}>{error}</div>}
            <div className={styles.cards}>
                {PLATFORMS.map((meta) => (
                    <PlatformCard
                        key={meta.id}
                        meta={meta}
                        label={t(meta.labelKey)}
                        placeholder={t(meta.placeholderKey)}
                        value={getInput(meta)}
                        buttonText={buttonText(meta)}
                        state={platformButtonState(connected[meta.id], waiting[meta.id])}
                        busy={connecting !== null}
                        onChange={(value) => setInput(meta.id, value)}
                        onToggle={() => void handleToggle(meta)}
                    />
                ))}

                <div className={styles.card}>
                    <div className={styles.cardHeader}>
                        <span className={styles.platformLabel}>
                            <svg
                                className={styles.platformIcon}
                                viewBox="0 0 24 24"
                                fill="currentColor"
                            >
                                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z" />
                            </svg>
                            {t('home.custom_source.label')}
                        </span>
                    </div>
                    <input
                        type="text"
                        placeholder={t('home.custom_source.placeholder')}
                        value={sourceURL}
                        onChange={(e) => setSourceURL(e.target.value)}
                    />
                    <p className={styles.hint}>{t('home.custom_source.hint')}</p>
                    <button
                        className="btn btn-primary"
                        onClick={handleOpenSource}
                        disabled={!isValidSourceURL(sourceURL)}
                    >
                        {t('home.custom_source.open')}
                    </button>
                </div>
            </div>

            {anyActive && (
                <button
                    className={`btn btn-primary ${styles.openChat}`}
                    onClick={() => navigate('/chat')}
                >
                    {t('home.open_chat')}
                </button>
            )}
        </div>
    );
}
