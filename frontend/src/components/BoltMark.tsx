const BOLT_ROWS = [
    '....####',
    '...####.',
    '..####..',
    '.#######',
    '#######.',
    '...###..',
    '..###...',
    '..##....',
    '.##.....',
    '.#......',
];

// The pixel bolt logo. The left pixel of each run is the lit edge.
export function BoltMark({ className }: { className?: string }) {
    return (
        <svg
            className={className}
            viewBox={`0 0 ${BOLT_ROWS[0].length} ${BOLT_ROWS.length}`}
            shapeRendering="crispEdges"
            aria-hidden="true"
        >
            {BOLT_ROWS.flatMap((row, y) =>
                row.split('').map((cell, x) =>
                    cell === '#' ? (
                        <rect
                            key={`${x}-${y}`}
                            x={x}
                            y={y}
                            width="1"
                            height="1"
                            fill={x === 0 || row[x - 1] !== '#' ? '#fffbd1' : '#ffeb3b'}
                        />
                    ) : null
                )
            )}
        </svg>
    );
}
