import type { PlatformMeta } from '@/platforms';

import styles from './Home.module.css';

export type PlatformState = 'idle' | 'connected' | 'waiting';

export function platformButtonState(connected: boolean | undefined, waiting: boolean | undefined): PlatformState {
    if (waiting) {
        return 'waiting';
    }

    return connected ? 'connected' : 'idle';
}

interface PlatformCardProps {
    meta: PlatformMeta;
    label: string;
    placeholder: string;
    value: string;
    buttonText: string;
    state: PlatformState;
    busy: boolean;
    onChange: (value: string) => void;
    onToggle: () => void;
}

export function PlatformCard({
    meta,
    label,
    placeholder,
    value,
    buttonText,
    state,
    busy,
    onChange,
    onToggle,
}: PlatformCardProps) {
    const active = state !== 'idle';

    return (
        <div className={styles.card}>
            <div className={styles.cardHeader}>
                <span className={styles.platformLabel}>
                    <meta.Icon className={styles.platformIcon} />
                    {label}
                </span>
                <span
                    className={`${styles.status} ${state === 'connected' ? styles.connected : ''} ${state === 'waiting' ? styles.waiting : ''}`}
                />
            </div>
            <input
                type="text"
                placeholder={placeholder}
                value={value}
                onChange={(e) => onChange(e.target.value)}
                onKeyDown={(e) => {
                    if (e.key === 'Enter' && value && !busy && !active) {
                        onToggle();
                    }
                }}
            />
            <button
                className={`btn ${active ? 'btn-danger' : 'btn-primary'}`}
                onClick={onToggle}
                disabled={busy || (!active && !value)}
            >
                {buttonText}
            </button>
        </div>
    );
}
