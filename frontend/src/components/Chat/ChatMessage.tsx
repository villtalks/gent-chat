import type { Platform } from '@bindings/ghost-chat/internal/chat/models.js';

import type { Badge, ChatMessage as ChatMessageType } from '@/types/chat';

import { useEffect, useState } from 'react';

import { platformMeta } from '@/platforms';

import styles from './ChatMessage.module.css';
import { renderFragments } from './renderFragments';

interface Props {
    message: ChatMessageType;
    hideBadges?: boolean;
    hideGifs?: boolean;
    showTimestamp?: boolean;
    showPlatformIcon?: boolean;
    showColon?: boolean;
    showAvatars?: boolean;
    fade?: boolean;
    fadeTimeout?: number;
    onFaded?: (id: string) => void;
}

function formatTime(ts: string) {
    const date = new Date(ts);

    if (Number.isNaN(date.getTime())) {
        return '';
    }

    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function isDark(hex: string): boolean {
    const r = Number.parseInt(hex.slice(1, 3), 16);
    const g = Number.parseInt(hex.slice(3, 5), 16);
    const b = Number.parseInt(hex.slice(5, 7), 16);

    return 0.299 * r + 0.587 * g + 0.114 * b < 128;
}

function BadgeList({ badges, hidden }: { badges: Badge[]; hidden?: boolean }) {
    if (hidden || !badges || badges.length === 0) {
        return null;
    }
    return (
        <span className={styles.badges}>
            {badges.map((badge) =>
                badge.url ? (
                    <img
                        key={`${badge.name}-${badge.version}`}
                        className={styles.badgeImg}
                        src={badge.url}
                        alt={badge.name}
                        title={badge.version || badge.name}
                    />
                ) : (
                    <span
                        key={`${badge.name}-${badge.version}`}
                        className={styles.badge}
                        title={badge.name}
                    >
                        {badge.name}
                    </span>
                )
            )}
        </span>
    );
}

function PlatformIcon({ platform }: { platform: Platform }) {
    const meta = platformMeta(platform);

    if (!meta) {
        return null;
    }

    return <meta.Icon className={styles.platformIcon} />;
}

export function ChatMessage({
    message,
    hideBadges,
    hideGifs,
    showTimestamp,
    showPlatformIcon,
    showColon = true,
    showAvatars = true,
    fade,
    fadeTimeout,
    onFaded,
}: Props) {
    const [fading, setFading] = useState(false);

    useEffect(() => {
        if (!fade || !fadeTimeout) {
            return;
        }
        const timer = setTimeout(() => setFading(true), fadeTimeout * 1000);
        return () => clearTimeout(timer);
    }, [fade, fadeTimeout]);

    const handleAnimationEnd = () => {
        if (fading) {
            onFaded?.(message.id);
        }
    };

    const wrapperClass = `${styles.message} ${fading ? styles.fade : ''}`;
    const body =
        message.fragments && message.fragments.length > 0 ? (
            renderFragments(message.fragments, { hideGifs })
        ) : (
            <span>{message.text}</span>
        );

    if (message.superChat) {
        const { headerColor, bodyColor, amount } = message.superChat;
        const headerTextColor = isDark(headerColor) ? '#ffffff' : 'rgba(0,0,0,0.87)';
        const bodyTextColor = isDark(bodyColor) ? '#ffffff' : 'rgba(0,0,0,0.87)';

        return (
            <div
                className={`${styles.message} ${styles.superChatWrapper} ${fading ? styles.fade : ''}`}
                onAnimationEnd={handleAnimationEnd}
            >
                <div
                    className={styles.superChatHeader}
                    style={{ background: headerColor, color: headerTextColor }}
                >
                    {message.avatar && (
                        <img
                            className={styles.avatar}
                            src={message.avatar}
                            alt=""
                        />
                    )}
                    <span className={styles.superChatUsername}>{message.username}</span>
                    <span className={styles.superChatAmount}>{amount}</span>
                </div>
                {message.fragments && message.fragments.length > 0 && (
                    <div
                        className={styles.superChatBody}
                        style={{ background: bodyColor, color: bodyTextColor }}
                    >
                        {body}
                    </div>
                )}
            </div>
        );
    }

    if (message.membershipEvent) {
        return (
            <div
                className={`${styles.message} ${styles.membershipWrapper} ${fading ? styles.fade : ''}`}
                onAnimationEnd={handleAnimationEnd}
            >
                <BadgeList
                    badges={message.badges}
                    hidden={hideBadges}
                />
                {message.avatar && (
                    <img
                        className={styles.avatar}
                        src={message.avatar}
                        alt=""
                    />
                )}
                <span className={styles.membershipUsername}>{message.username}</span>
                <span className={styles.membershipText}>{body}</span>
            </div>
        );
    }

    return (
        <div
            className={`${wrapperClass} ${message.isAction ? styles.action : ''}`}
            onAnimationEnd={handleAnimationEnd}
        >
            {showTimestamp && message.timestamp && (
                <span className={styles.timestamp}>{formatTime(message.timestamp)}</span>
            )}
            {showPlatformIcon && <PlatformIcon platform={message.platform} />}
            {showAvatars && message.avatar && (
                <img
                    className={styles.avatar}
                    src={message.avatar}
                    alt=""
                />
            )}
            <BadgeList
                badges={message.badges}
                hidden={hideBadges}
            />
            <span
                className={styles.username}
                style={{ color: message.color || 'var(--color-accent)' }}
            >
                {message.username}
            </span>
            <span className={styles.separator}>{message.isAction ? ' ' : showColon ? ': ' : ' '}</span>
            <span className={styles.text}>{body}</span>
        </div>
    );
}
