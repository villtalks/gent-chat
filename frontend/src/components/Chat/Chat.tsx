import type { Platform } from '@bindings/ghost-chat/internal/chat/models.js';
import type React from 'react';

import type { ChatMessage as ChatMessageType } from '@/types/chat';

import { ToggleVanish } from '@bindings/ghost-chat/app.js';
import { Events } from '@wailsio/runtime';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';

import { fadePolicy, shouldDisplay } from '@/filter/messageFilter';
import { PLATFORMS } from '@/platforms';
import { useConfigStore } from '@/stores/config';
import { useConnectionStore } from '@/stores/connection';
import { useViewersStore } from '@/stores/viewers';
import { getThemeById, themeToCSS } from '@/types/theme';

import styles from './Chat.module.css';
import { ChatMessage } from './ChatMessage';
import { EventMessage } from './EventMessage';
import { ViewerBar } from './ViewerBar';

const MAX_MESSAGES = 500;

export function Chat() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const config = useConfigStore((s) => s.config);
    const [messages, setMessages] = useState<ChatMessageType[]>([]);
    const [autoScroll, setAutoScroll] = useState(true);
    const autoScrollRef = useRef(true);
    const userScrollingRef = useRef(false);
    const [hidden, setHidden] = useState<Partial<Record<Platform, boolean>>>({});
    const connectedMap = useConnectionStore((s) => s.connected);
    const waitingMap = useConnectionStore((s) => s.waiting);
    const viewerCounts = useViewersStore((s) => s.counts);
    const connectedPlatforms = PLATFORMS.filter((p) => connectedMap[p.id]);
    const activePlatforms = PLATFORMS.filter((p) => connectedMap[p.id] || waitingMap[p.id]).map((p) => p.id);
    const connected = connectedPlatforms.length > 0;
    const anyWaiting = PLATFORMS.some((p) => waitingMap[p.id]);
    const connectedCount = connectedPlatforms.length;
    const messagesRef = useRef<HTMLDivElement>(null);

    const activeThemeId = config?.theme?.active_theme_id ?? 'default';
    const customThemes = config?.theme?.custom_themes ?? [];
    const theme = getThemeById(activeThemeId, customThemes);
    const themeCSSVars = themeToCSS(theme);

    const topToBottom = theme.top_to_bottom ?? false;

    const hideBadges = config?.twitch?.hide_badges ?? false;
    const hideGifs = config?.twitch?.hide_gifs ?? false;
    const showTimestamp = config?.general?.show_timestamps ?? false;

    useEffect(() => {
        const cancelMessage = Events.On('chat:message', (ev) => {
            const msg = ev.data as ChatMessageType;
            const cfg = useConfigStore.getState().config;

            if (!shouldDisplay(msg, cfg)) {
                return;
            }

            setMessages((prev) => {
                const next = [...prev, msg];
                if (next.length > MAX_MESSAGES) {
                    return next.slice(next.length - MAX_MESSAGES);
                }
                return next;
            });
        });

        const cancelClear = Events.On('chat:clear', (ev) => {
            const username = ev.data as string;

            setMessages((prev) => prev.filter((m) => m.username.toLowerCase() !== username.toLowerCase()));
        });

        const cancelDelete = Events.On('chat:delete-message', (ev) => {
            const msgId = ev.data as string;

            setMessages((prev) => prev.filter((m) => m.id !== msgId));
        });

        return () => {
            cancelMessage();
            cancelClear();
            cancelDelete();
        };
    }, []);

    useEffect(() => {
        if (autoScrollRef.current && !userScrollingRef.current && messagesRef.current) {
            messagesRef.current.scrollTop = topToBottom ? 0 : messagesRef.current.scrollHeight;
        }
    }, [messages, topToBottom]);

    useEffect(() => {
        const el = messagesRef.current;

        if (!el) {
            return;
        }

        let scrollTimer: ReturnType<typeof setTimeout>;

        const handleWheel = (e: WheelEvent) => {
            const scrollingAway = topToBottom ? e.deltaY > 0 : e.deltaY < 0;

            if (scrollingAway) {
                userScrollingRef.current = true;
                autoScrollRef.current = false;
                setAutoScroll(false);

                clearTimeout(scrollTimer);

                scrollTimer = setTimeout(() => {
                    userScrollingRef.current = false;
                }, 150);
            }
        };

        el.addEventListener('wheel', handleWheel, { passive: true });

        return () => {
            el.removeEventListener('wheel', handleWheel);
            clearTimeout(scrollTimer);
        };
    }, [topToBottom]);

    const handleScroll = () => {
        const el = messagesRef.current;

        if (!el) {
            return;
        }

        let atEdge: boolean;

        if (topToBottom) {
            atEdge = el.scrollTop < 40;
        } else {
            atEdge = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
        }

        if (atEdge && !autoScrollRef.current) {
            autoScrollRef.current = true;
            setAutoScroll(true);
        }
    };

    const displayMessages = topToBottom ? [...messages].toReversed() : messages;

    const visibleMessages = displayMessages.filter((m) => !hidden[m.platform]);

    const viewerBar = (
        <ViewerBar
            platforms={activePlatforms}
            counts={viewerCounts}
            showTotal={config?.general?.show_viewer_count ?? true}
            showPlatforms={config?.general?.show_platform_viewers ?? true}
            style={themeCSSVars as React.CSSProperties}
        />
    );

    return (
        <div className={styles.chat}>
            <div className={styles.header}>
                <button
                    className="btn btn-ghost"
                    onClick={() => navigate('/')}
                >
                    <svg
                        width="14"
                        height="14"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    >
                        <polyline points="15 18 9 12 15 6" />
                    </svg>
                    {t('chat.back')}
                </button>
                {connectedCount > 1 && (
                    <div className={styles.filters}>
                        {connectedPlatforms.map((meta) => (
                            <button
                                key={meta.id}
                                className={`${styles.filterBtn} ${hidden[meta.id] ? '' : styles.filterActive}`}
                                onClick={() => setHidden((prev) => ({ ...prev, [meta.id]: !prev[meta.id] }))}
                                title={t('chat.toggle_platform', { platform: t(meta.labelKey) })}
                            >
                                {meta.short}
                            </button>
                        ))}
                    </div>
                )}
                <button
                    className={styles.vanishBtn}
                    onClick={() => void ToggleVanish()}
                    title={t('chat.vanish')}
                >
                    <svg
                        width="14"
                        height="14"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    >
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                        <circle
                            cx="12"
                            cy="12"
                            r="3"
                        />
                    </svg>
                </button>
                {!connected && (
                    <span className={styles.disconnected}>
                        {anyWaiting ? t('home.waiting') : t('chat.disconnected')}
                    </span>
                )}
            </div>
            {!topToBottom && viewerBar}
            <div
                ref={messagesRef}
                className={styles.messages}
                style={themeCSSVars as React.CSSProperties}
                onScroll={handleScroll}
            >
                {!topToBottom && <div className={styles.messagesSpacer} />}
                {visibleMessages.length === 0
                    ? config?.general?.show_waiting_message !== false && (
                          <div className={styles.empty}>
                              <span>{t('chat.waiting')}</span>
                          </div>
                      )
                    : visibleMessages.map((msg) => {
                          const { fade, timeoutSeconds } = fadePolicy(msg.platform, config);

                          return msg.eventType ? (
                              <EventMessage
                                  key={msg.id}
                                  message={msg}
                                  hideGifs={hideGifs}
                                  showTimestamp={showTimestamp}
                                  fade={fade}
                                  fadeTimeout={timeoutSeconds}
                                  onFaded={(id) => setMessages((prev) => prev.filter((m) => m.id !== id))}
                              />
                          ) : (
                              <ChatMessage
                                  key={msg.id}
                                  message={msg}
                                  hideBadges={hideBadges}
                                  hideGifs={hideGifs}
                                  showTimestamp={showTimestamp}
                                  showPlatformIcon={connectedCount > 1}
                                  showColon={theme.show_colon}
                                  showAvatars={theme.show_avatars}
                                  fade={fade}
                                  fadeTimeout={timeoutSeconds}
                                  onFaded={(id) => setMessages((prev) => prev.filter((m) => m.id !== id))}
                              />
                          );
                      })}
            </div>
            {topToBottom && viewerBar}
            {!autoScroll && (
                <button
                    className={`btn btn-ghost ${styles.scrollBtn}`}
                    onClick={() => {
                        autoScrollRef.current = true;
                        setAutoScroll(true);
                        messagesRef.current?.scrollTo({
                            top: topToBottom ? 0 : messagesRef.current.scrollHeight,
                            behavior: 'smooth',
                        });
                    }}
                >
                    {topToBottom ? t('chat.scroll_to_top') : t('chat.scroll_to_bottom')}
                </button>
            )}
        </div>
    );
}
