import type { Platform } from '@bindings/ghost-chat/internal/chat/models.js';
import type { CSSProperties } from 'react';

import type { ViewerCounts } from '@/stores/viewers';

import { PLATFORMS } from '@/platforms';
import { totalOf } from '@/stores/viewers';

import styles from './ViewerBar.module.css';

export function formatCount(n: number): string {
    return Math.max(0, Math.floor(n))
        .toString()
        .replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}

interface ViewerBarProps {
    platforms: Platform[];
    counts: ViewerCounts;
    showTotal: boolean;
    showPlatforms: boolean;
    style?: CSSProperties;
}

// The live viewer row of the overlay: total first, then one chip per connected
// platform. It draws text only, so the window behind it stays see-through.
export function ViewerBar({ platforms, counts, showTotal, showPlatforms, style }: ViewerBarProps) {
    if (!showTotal || platforms.length === 0) {
        return null;
    }

    const shown = PLATFORMS.filter((meta) => platforms.includes(meta.id));
    const total = totalOf(
        counts,
        shown.map((meta) => meta.id)
    );

    return (
        <div
            className={styles.bar}
            style={style}
            data-viewer-bar=""
        >
            <span className={styles.total}>
                <span className={styles.dot} />
                <span>{formatCount(total)}</span>
            </span>
            {showPlatforms &&
                shown.map((meta) => {
                    const entry = counts[meta.id];

                    return (
                        <span
                            key={meta.id}
                            className={styles.chip}
                            data-platform={meta.id}
                        >
                            <meta.Icon className={styles.icon} />
                            <span>{entry?.live ? formatCount(entry.count) : '–'}</span>
                        </span>
                    );
                })}
        </div>
    );
}
