import type { Config } from '@bindings/ghost-chat/internal/config/models.js';
import type { JSX } from 'react';

import { Platform } from '@bindings/ghost-chat/internal/chat/models.js';

type IconProps = { className?: string };

export interface PlatformMeta {
    id: Platform;
    labelKey: string;
    placeholderKey: string;
    short: string;
    color: string;
    Icon: (props: IconProps) => JSX.Element;
    savedInput: (config: Config | null | undefined) => string;
    supportsWaiting: boolean;
}

const YOUTUBE_PATH =
    'M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z';

function TwitchIcon({ className }: IconProps) {
    return (
        <svg
            className={className}
            viewBox="0 0 24 24"
            fill="#9146ff"
            aria-hidden="true"
        >
            <path d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z" />
        </svg>
    );
}

function YouTubeIcon({ className }: IconProps) {
    return (
        <svg
            className={className}
            viewBox="0 0 24 24"
            fill="#ff0000"
            aria-hidden="true"
        >
            <path d={YOUTUBE_PATH} />
        </svg>
    );
}

// The YouTube play button inside a phone outline: the vertical stream.
function YouTubeVerticalIcon({ className }: IconProps) {
    return (
        <svg
            className={className}
            viewBox="0 0 24 24"
            aria-hidden="true"
        >
            <rect
                x="5.5"
                y="1"
                width="13"
                height="22"
                rx="3"
                fill="none"
                stroke="#ff0000"
                strokeWidth="2"
            />
            <path
                d="M10 8.5v7l6-3.5z"
                fill="#ff0000"
            />
        </svg>
    );
}

function KickIcon({ className }: IconProps) {
    return (
        <svg
            className={className}
            viewBox="0 0 512 512"
            fill="#53fc18"
            aria-hidden="true"
        >
            <path d="M37 .036h164.448v113.621h54.71v-56.82h54.731V.036h164.448v170.777h-54.73v56.82h-54.711v56.8h54.71v56.82h54.73V512.03H310.89v-56.82h-54.73v-56.8h-54.711v113.62H37V.036z" />
        </svg>
    );
}

// A plain music note in TikTok's pink with a cyan echo; not the TikTok logo.
function TikTokIcon({ className }: IconProps) {
    const note = 'M14 3v10.2a3.8 3.8 0 1 1-2.6-3.6V3zm0 0c.3 2.3 1.9 4 4.4 4.3';

    return (
        <svg
            className={className}
            viewBox="0 0 24 24"
            fill="none"
            strokeWidth="2.6"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
        >
            <path
                d={note}
                stroke="#25f4ee"
                transform="translate(-1.2 -0.8)"
            />
            <path
                d={note}
                stroke="#ff0050"
                transform="translate(1 0.8)"
            />
        </svg>
    );
}

export const PLATFORMS: PlatformMeta[] = [
    {
        id: Platform.PlatformTwitch,
        labelKey: 'home.twitch',
        placeholderKey: 'home.placeholder.channel',
        short: 'T',
        color: '#9146ff',
        Icon: TwitchIcon,
        savedInput: (c) => c?.twitch?.default_channel ?? '',
        supportsWaiting: false,
    },
    {
        id: Platform.PlatformYouTube,
        labelKey: 'home.youtube',
        placeholderKey: 'home.placeholder.channel_or_url',
        short: 'YT',
        color: '#ff0000',
        Icon: YouTubeIcon,
        savedInput: (c) => c?.youtube?.channel_id ?? '',
        supportsWaiting: true,
    },
    {
        id: Platform.PlatformYouTubeVertical,
        labelKey: 'home.youtube_vertical',
        placeholderKey: 'home.placeholder.youtube_vertical',
        short: 'YTV',
        color: '#ff0000',
        Icon: YouTubeVerticalIcon,
        savedInput: (c) => c?.youtube_vertical?.channel_id ?? '',
        supportsWaiting: true,
    },
    {
        id: Platform.PlatformKick,
        labelKey: 'home.kick',
        placeholderKey: 'home.placeholder.kick_channel',
        short: 'K',
        color: '#53fc18',
        Icon: KickIcon,
        savedInput: (c) => c?.kick?.default_channel ?? '',
        supportsWaiting: false,
    },
    {
        id: Platform.PlatformTikTok,
        labelKey: 'home.tiktok',
        placeholderKey: 'home.placeholder.tiktok',
        short: 'TT',
        color: '#ff0050',
        Icon: TikTokIcon,
        savedInput: (c) => c?.tiktok?.default_channel ?? '',
        supportsWaiting: true,
    },
];

export function platformMeta(id: Platform): PlatformMeta | undefined {
    return PLATFORMS.find((p) => p.id === id);
}
