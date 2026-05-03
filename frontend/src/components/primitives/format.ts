/**
 * Number formatting helpers for the editorial trading-floor look.
 * All numerics render in JetBrains Mono with tabular numerals — keep these
 * pure-string helpers so callers can compose with their own JSX.
 */

const usd = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const integer = new Intl.NumberFormat('en-US');

const signed = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
  signDisplay: 'always',
});

export const formatMoney = (value: number | null | undefined): string => {
  if (value == null || Number.isNaN(value)) return '0.00';
  return usd.format(value);
};

export const formatSignedMoney = (value: number | null | undefined): string => {
  if (value == null || Number.isNaN(value)) return '+0.00';
  // Intl uses ASCII '-'; the design uses the typographic minus '−' for losses.
  const formatted = signed.format(value);
  return formatted.replace('-', '−');
};

export const formatPercent = (value: number | null | undefined, fractionDigits = 2): string => {
  if (value == null || Number.isNaN(value)) return '+0.00%';
  const sign = value > 0 ? '+' : value < 0 ? '−' : '+';
  return `${sign}${Math.abs(value).toFixed(fractionDigits)}%`;
};

export const formatInteger = (value: number | null | undefined): string => {
  if (value == null || Number.isNaN(value)) return '0';
  return integer.format(value);
};

export const formatTimestamp = (date: Date = new Date()): string => {
  const dateStr = date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
  const timeStr = date.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
  return `${dateStr} · ${timeStr} ET`;
};
