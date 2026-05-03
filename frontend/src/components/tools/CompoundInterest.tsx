import React, { useState, ChangeEvent } from 'react';
import { formatMoney, formatSignedMoney } from '../primitives/format';
import ToolsTabs from './ToolsTabs';
import GrowthChart, { GrowthSeriesPoint } from './GrowthChart';

interface CompoundInterestResult {
  futureValue: number;
  totalContributions: number;
  interestEarned: number;
  series: GrowthSeriesPoint[];
}

const CompoundInterest: React.FC = () => {
  const [startingBalance, setStartingBalance] = useState<string>('');
  const [monthlyContribution, setMonthlyContribution] = useState<string>('');
  const [years, setYears] = useState<string>('');
  const [interestRate, setInterestRate] = useState<string>('');
  const [result, setResult] = useState<CompoundInterestResult | null>(null);

  const calculateCompoundInterest = () => {
    const P = parseFloat(startingBalance) || 0;
    const PMT = parseFloat(monthlyContribution) || 0;
    const t = parseFloat(years) || 0;
    const r = parseFloat(interestRate) / 100 || 0;
    const n = 12;
    const totalMonths = Math.max(0, Math.round(n * t));

    // Step the projection forward month-by-month so the chart can render the
    // curve between integer years (and so partial-year inputs still work).
    const series: GrowthSeriesPoint[] = [
      { year: 0, balance: P, contributions: P },
    ];
    let balance = P;
    let contributions = P;
    const monthlyRate = r / n;
    const samplesPerYear = 4; // quarterly samples — keeps the path smooth without bloating the SVG
    const sampleEveryMonths = Math.max(1, Math.round(n / samplesPerYear));

    for (let m = 1; m <= totalMonths; m += 1) {
      balance = balance * (1 + monthlyRate) + PMT;
      contributions += PMT;
      if (m % sampleEveryMonths === 0 || m === totalMonths) {
        series.push({ year: m / n, balance, contributions });
      }
    }

    const futureValue = balance;
    const totalContributions = contributions;
    const interestEarned = futureValue - totalContributions;

    setResult({ futureValue, totalContributions, interestEarned, series });
  };

  return (
    <div className="container-narrow" style={{ paddingTop: 24 }}>
      <header className="page-header">
        <div>
          <div className="eyebrow" style={{ marginBottom: 6 }}>
            Tools · projection
          </div>
          <h1>Compound interest</h1>
        </div>
      </header>

      <ToolsTabs />

      <div className="panel" style={{ padding: 28 }}>
        <p className="muted" style={{ marginTop: 0, marginBottom: 24, fontSize: 14 }}>
          Project the future value of an investment with monthly contributions.
        </p>

        <div className="form-group">
          <label htmlFor="startingBalance" className="form-label">
            Initial investment ($)
          </label>
          <input
            type="number"
            id="startingBalance"
            className="input mono"
            value={startingBalance}
            onChange={(e: ChangeEvent<HTMLInputElement>) => setStartingBalance(e.target.value)}
            placeholder="1000"
            step="0.01"
          />
        </div>

        <div className="form-group">
          <label htmlFor="monthlyContribution" className="form-label">
            Monthly contribution ($)
          </label>
          <input
            type="number"
            id="monthlyContribution"
            className="input mono"
            value={monthlyContribution}
            onChange={(e: ChangeEvent<HTMLInputElement>) => setMonthlyContribution(e.target.value)}
            placeholder="100"
            step="0.01"
          />
        </div>

        <div className="form-group">
          <label htmlFor="years" className="form-label">
            Time period (years)
          </label>
          <input
            type="number"
            id="years"
            className="input mono"
            value={years}
            onChange={(e: ChangeEvent<HTMLInputElement>) => setYears(e.target.value)}
            placeholder="10"
            step="0.1"
          />
        </div>

        <div className="form-group">
          <label htmlFor="interestRate" className="form-label">
            Annual interest rate (%)
          </label>
          <input
            type="number"
            id="interestRate"
            className="input mono"
            value={interestRate}
            onChange={(e: ChangeEvent<HTMLInputElement>) => setInterestRate(e.target.value)}
            placeholder="7"
            step="0.01"
          />
        </div>

        <button
          type="button"
          onClick={calculateCompoundInterest}
          className="btn btn-primary btn-block"
        >
          Calculate
        </button>

        {result && (
          <div
            style={{
              marginTop: 28,
              padding: '20px 22px',
              border: '1px solid var(--hairline)',
              borderRadius: 8,
              background: 'var(--canvas)',
            }}
          >
            <div className="eyebrow" style={{ marginBottom: 12 }}>
              Result
            </div>
            <div className="display-num" style={{ fontSize: 44 }}>
              <span className="currency">$</span>
              <span className="num">{formatMoney(result.futureValue)}</span>
            </div>
            <div
              className="mono"
              style={{ fontSize: 13, marginTop: 14, display: 'grid', rowGap: 6 }}
            >
              <Row label="Total contributions" value={`$${formatMoney(result.totalContributions)}`} />
              <Row
                label="Interest earned"
                value={`${formatSignedMoney(result.interestEarned)}`}
                tone={result.interestEarned >= 0 ? 'gain' : 'loss'}
              />
            </div>

            {result.series.length > 1 && (
              <div style={{ marginTop: 20 }}>
                <GrowthChart series={result.series} />
              </div>
            )}

            <div
              className="mono"
              style={{
                marginTop: 16,
                fontSize: 11,
                color: 'var(--ink-muted)',
                paddingTop: 12,
                borderTop: '1px solid var(--hairline)',
              }}
            >
              Compounded monthly: balance ← balance × (1 + r/12) + PMT, every month for nt months.
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

const Row: React.FC<{ label: string; value: string; tone?: 'gain' | 'loss' }> = ({
  label,
  value,
  tone,
}) => (
  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
    <span style={{ color: 'var(--ink-muted)' }}>{label}</span>
    <span
      style={{
        color: tone === 'gain' ? 'var(--gain)' : tone === 'loss' ? 'var(--loss)' : 'var(--ink)',
        fontWeight: 500,
      }}
    >
      {value}
    </span>
  </div>
);

export default CompoundInterest;
