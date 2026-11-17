import type { Platform } from '@bindings/ghost-chat/internal/chat/models.js';

import type { ViewerUpdate } from '@/stores/viewers';

import { ExpandForSettings, GetViewers, ShrinkToChat } from '@bindings/ghost-chat/app.js';
import { Events } from '@wailsio/runtime';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { HashRouter, Routes, Route } from 'react-router-dom';

import { AutoConnect } from '@/components/AutoConnect';
import { Chat } from '@/components/Chat/Chat';
import { CustomSource } from '@/components/CustomSource/CustomSource';
import { Home } from '@/components/Home/Home';
import { Settings } from '@/components/Settings/Settings';
import { ThemePreview } from '@/components/Settings/ThemePreview';
import { TitleBar } from '@/components/TitleBar/TitleBar';
import { useConfigStore } from '@/stores/config';
import { useConnectionStore } from '@/stores/connection';
import { useViewersStore } from '@/stores/viewers';

import styles from './App.module.css';

function App() {
    const [settingsOpen, setSettingsOpen] = useState(false);
    const [settingsTab, setSettingsTab] = useState('general');
    const [vanished, setVanished] = useState(false);
    const { i18n } = useTranslation();
    const load = useConfigStore((s) => s.load);
    const loaded = useConfigStore((s) => s.loaded);
    const language = useConfigStore((s) => s.config?.general?.language);
    const setConnected = useConnectionStore((s) => s.setConnected);
    const setWaiting = useConnectionStore((s) => s.setWaiting);

    const toggleSettings = () => {
        if (settingsOpen) {
            void ShrinkToChat();
        } else {
            void ExpandForSettings();
        }
        setSettingsOpen((v) => !v);
    };

    useEffect(() => {
        void load();
    }, [load]);

    useEffect(() => {
        const cancelConnected = Events.On('chat:connected', (ev) => {
            const { platform } = ev.data as { platform: Platform };
            setConnected(platform, true);
        });

        const cancelWaiting = Events.On('chat:waiting', (ev) => {
            const { platform } = ev.data as { platform: Platform };
            setWaiting(platform, true);
        });

        const cancelDisconnected = Events.On('chat:disconnected', (ev) => {
            const { platform } = ev.data as { platform: Platform };
            setConnected(platform, false);
        });

        return () => {
            cancelConnected();
            cancelWaiting();
            cancelDisconnected();
        };
    }, [setConnected, setWaiting]);

    useEffect(() => {
        const cancelViewers = Events.On('viewers:update', (ev) => {
            useViewersStore.getState().apply(ev.data as ViewerUpdate);
        });

        void GetViewers()
            .then((latest) => useViewersStore.getState().seed(latest as ViewerUpdate[] | null))
            .catch(() => {});

        return () => {
            cancelViewers();
        };
    }, []);

    useEffect(() => {
        const cancelVanish = Events.On('vanish:toggle', (ev) => {
            setVanished(ev.data as boolean);
        });

        return () => {
            cancelVanish();
        };
    }, []);

    useEffect(() => {
        if (vanished) {
            document.documentElement.setAttribute('data-vanished', '');
        } else {
            document.documentElement.removeAttribute('data-vanished');
        }
    }, [vanished]);

    useEffect(() => {
        if (language && language !== i18n.language) {
            void i18n.changeLanguage(language);
        }
    }, [language, i18n]);

    if (!loaded) {
        return null;
    }

    return (
        <HashRouter>
            <AutoConnect />
            <div
                className={`${styles.app} ${settingsOpen ? styles.withSettings : ''} ${vanished ? styles.vanished : ''}`}
                data-vanished={vanished || undefined}
            >
                <TitleBar
                    onSettingsToggle={toggleSettings}
                    settingsOpen={settingsOpen}
                />
                <div className={styles.body}>
                    {settingsOpen && !vanished && (
                        <div className={styles.settingsPanel}>
                            <Settings onTabChange={setSettingsTab} />
                        </div>
                    )}
                    <div className={styles.main}>
                        {settingsOpen && !vanished && settingsTab === 'themes' && <ThemePreview />}
                        <div
                            style={{
                                display: settingsOpen && !vanished && settingsTab === 'themes' ? 'none' : 'contents',
                            }}
                        >
                            <Routes>
                                <Route
                                    path="/"
                                    element={<Home />}
                                />
                                <Route
                                    path="/chat"
                                    element={<Chat />}
                                />
                                <Route
                                    path="/source"
                                    element={<CustomSource />}
                                />
                            </Routes>
                        </div>
                    </div>
                </div>
            </div>
        </HashRouter>
    );
}

export default App;
