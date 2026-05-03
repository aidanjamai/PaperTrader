import React, { useMemo, useState } from 'react';
import { HistoricalSeriesPoint } from '../../types';
import { formatMoney } from '../primitives/format';

interface PriceChartProps {
  points: HistoricalSeriesPoint[];
}

const VIEWBOX_WIDTH = 720;
const VIEWBOX_HEIGHT = 280;
const PADDING_TOP = 12;
const PADDING_RIGHT = 12;
const PADDING_BOTTOM = 28;
const PADDING_LEFT = 60;
const PLOT_WIDTH = VIEWBOX_WIDTH - PADDING_LEFT - PADDING_RIGHT;
const PLOT_HEIGHT = VIEWBOX_HEIGHT - PADDING_TOP - PADDING_BOTTOM;

const formatAxisMoney = (value: number): string => {
  if (value >= 1_000) return `$${value.toFixed(0)}`;
  return `$${value.toFixed(2)}`;
};

const formatDateLabel = (iso: string): string => {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
};

const PriceChart: React.FC<PriceChartProps> = ({ points }) => {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  const chart = useMemo(() => {
    if (points.length < 2) return null;

    const closes = points.map((p) => p.close);
    const yMin = Math.min(...closes);
    const yMax = Math.max(...closes);
    // Pad the y range by 4% so the line doesn't ride the edges.
    const pad = (yMax - yMin) * 0.04 || yMax * 0.02 || 1;
    const yLo = yMin - pad;
    const yHi = yMax + pad;

    const xCount = points.length - 1;
    const xScale = (i: number): number =>
      PADDING_LEFT + (i / xCount) * PLOT_WIDTH;
    const yScale = (value: number): number =>
      PADDING_TOP + PLOT_HEIGHT - ((value - yLo) / (yHi - yLo)) * PLOT_HEIGHT;

    const linePts = points.map(
      (p, i) => `${xScale(i).toFixed(2)},${yScale(p.close).toFixed(2)}`
    );
    const linePath = `M${linePts.join(' L')}`;
    const areaPath = `M${PADDING_LEFT},${PADDING_TOP + PLOT_HEIGHT} L${linePts.join(' L')} L${(
      PADDING_LEFT +
      PLOT_WIDTH
    ).toFixed(2)},${PADDING_TOP + PLOT_HEIGHT} Z`;

    const yTickCount = 4;
    const yTicks = Array.from({ length: yTickCount + 1 }, (_, i) => {
      const value = yLo + ((yHi - yLo) / yTickCount) * i;
      return { value, y: yScale(value) };
    });

    // Aim for ~6 evenly spaced labels with the last point pinned. We pin the
    // end first, then walk backwards by stride so the right-most label is
    // always the actual range end (no orphan tick crammed against it).
    // The loop guard `i > 0` is intentional: index 0 is force-added afterwards
    // so the start is always labelled regardless of where stride lands.
    const targetCount = 6;
    const stride = Math.max(1, Math.floor((points.length - 1) / (targetCount - 1)));
    const indices = new Set<number>();
    indices.add(points.length - 1);
    for (let i = points.length - 1 - stride; i > 0; i -= stride) {
      indices.add(i);
    }
    indices.add(0);

    // Drop any tick that would render too close to its neighbour. ~52px at the
    // current viewBox covers a 6-char label like "Apr 28" without overlap.
    const minPx = 52;
    const sorted = Array.from(indices).sort((a, b) => a - b);
    const xTicks: { i: number; x: number; date: string }[] = [];
    let lastX = -Infinity;
    sorted.forEach((i, idx) => {
      const x = xScale(i);
      const isFirst = idx === 0;
      const isLast = idx === sorted.length - 1;
      if (isFirst || isLast || x - lastX >= minPx) {
        xTicks.push({ i, x, date: points[i].date });
        lastX = x;
      }
    });
    // Final pass: if the last two ticks ended up too close, drop the second-
    // last so the range-end label always wins.
    if (xTicks.length >= 2) {
      const tail = xTicks[xTicks.length - 1];
      const prev = xTicks[xTicks.length - 2];
      if (tail.x - prev.x < minPx) {
        xTicks.splice(xTicks.length - 2, 1);
      }
    }

    const trend: 'gain' | 'loss' | 'flat' =
      points[points.length - 1].close > points[0].close
        ? 'gain'
        : points[points.length - 1].close < points[0].close
        ? 'loss'
        : 'flat';

    return {
      linePath,
      areaPath,
      yTicks,
      xTicks,
      xScale,
      yScale,
      trend,
    };
  }, [points]);

  if (!chart) {
    return (
      <div className="empty-state">
        <span className="empty-title">Not enough data to chart</span>
      </div>
    );
  }

  const stroke =
    chart.trend === 'gain'
      ? 'var(--gain)'
      : chart.trend === 'loss'
      ? 'var(--loss)'
      : 'var(--accent)';
  const fill =
    chart.trend === 'gain'
      ? 'var(--gain-tint)'
      : chart.trend === 'loss'
      ? 'var(--loss-tint)'
      : 'var(--accent-tint)';

  const handleMove = (event: React.MouseEvent<SVGSVGElement>) => {
    const svg = event.currentTarget;
    const rect = svg.getBoundingClientRect();
    const ratio = (event.clientX - rect.left) / rect.width;
    const x = ratio * VIEWBOX_WIDTH;
    let nearest = 0;
    let nearestDist = Infinity;
    points.forEach((_, i) => {
      const px = chart.xScale(i);
      const dist = Math.abs(px - x);
      if (dist < nearestDist) {
        nearest = i;
        nearestDist = dist;
      }
    });
    setHoverIndex(nearest);
  };

  const hovered = hoverIndex == null ? null : points[hoverIndex];

  return (
    <div>
      <div
        className="eyebrow"
        style={{ marginBottom: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}
      >
        <span>Daily close</span>
        <span
          className="mono"
          style={{ fontSize: 11, color: 'var(--ink-muted)', textTransform: 'none', letterSpacing: 0 }}
        >
          {hovered
            ? `${formatDateLabel(hovered.date)} · $${formatMoney(hovered.close)}`
            : `${points.length} trading days`}
        </span>
      </div>

      <svg
        viewBox={`0 0 ${VIEWBOX_WIDTH} ${VIEWBOX_HEIGHT}`}
        role="img"
        aria-label="Daily close price over the selected range"
        style={{
          width: '100%',
          height: 'auto',
          display: 'block',
          fontFamily: '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
        }}
        onMouseMove={handleMove}
        onMouseLeave={() => setHoverIndex(null)}
      >
        {chart.yTicks.map((tick, idx) => (
          <g key={`y-${idx}`}>
            <line
              x1={PADDING_LEFT}
              x2={PADDING_LEFT + PLOT_WIDTH}
              y1={tick.y}
              y2={tick.y}
              stroke="var(--hairline)"
              strokeWidth={1}
              strokeDasharray={idx === 0 ? undefined : '2 4'}
            />
            <text
              x={PADDING_LEFT - 8}
              y={tick.y + 3}
              fill="var(--ink-muted)"
              fontSize={10}
              textAnchor="end"
            >
              {formatAxisMoney(tick.value)}
            </text>
          </g>
        ))}

        {chart.xTicks.map((tick) => (
          <text
            key={`x-${tick.i}`}
            x={tick.x}
            y={PADDING_TOP + PLOT_HEIGHT + 16}
            fill="var(--ink-muted)"
            fontSize={10}
            textAnchor="middle"
          >
            {formatDateLabel(tick.date)}
          </text>
        ))}

        <path d={chart.areaPath} fill={fill} stroke="none" />
        <path d={chart.linePath} fill="none" stroke={stroke} strokeWidth={1.75} />

        {hovered && hoverIndex != null && (
          <g>
            <line
              x1={chart.xScale(hoverIndex)}
              x2={chart.xScale(hoverIndex)}
              y1={PADDING_TOP}
              y2={PADDING_TOP + PLOT_HEIGHT}
              stroke="var(--ink-muted)"
              strokeWidth={1}
              strokeDasharray="2 3"
            />
            <circle
              cx={chart.xScale(hoverIndex)}
              cy={chart.yScale(hovered.close)}
              r={3.5}
              fill={stroke}
            />
          </g>
        )}
      </svg>
    </div>
  );
};

export default PriceChart;
