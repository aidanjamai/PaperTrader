import React from 'react';
import { Link } from 'react-router-dom';

const Home: React.FC = () => {
  return (
    <div className="container-narrow">
      <section className="hero">
        <div>
          <div className="eyebrow" style={{ marginBottom: 18 }}>
            No. 01 · A trading sandbox
          </div>
          <h1>
            Practice trading.<br />
            Without the practice <span className="accent">losses.</span>
          </h1>
          <p className="lede">
            Open a paper account, get $100,000 in simulated capital, and trade real markets at
            real prices. Build the muscle memory before you build the position.
          </p>
          <div className="ctas">
            <Link to="/register" className="btn btn-primary btn-lg">
              Start paper trading
              <svg className="ico" viewBox="0 0 16 16" aria-hidden="true">
                <path d="M3 8h10" />
                <path d="M9 4l4 4-4 4" />
              </svg>
            </Link>
            <Link to="/markets" className="btn btn-ghost btn-lg">
              Browse markets
            </Link>
          </div>
          <div className="meta-line">
            <span className="mono">$100k</span> simulated capital
            <span className="dot" />
            <span className="mono">9,400+</span> tickers
            <span className="dot" />
            <span>Free, forever</span>
          </div>
        </div>

        <aside className="live-card" aria-label="Sample portfolio">
          <div className="head">
            <div className="label">Portfolio · sample</div>
            <div className="live">
              <span className="live-dot" /> LIVE · 14:32 ET
            </div>
          </div>
          <div className="value">
            <span className="currency">$</span>
            <span className="num">127,432.18</span>
          </div>
          <div className="row-meta">
            <span className="pill pill-gain">+$1,284.32</span>
            <span className="pill pill-gain">+1.02%</span>
            <span>· today</span>
          </div>
          <div className="chart-wrap">
            <svg
              viewBox="0 0 420 110"
              width="100%"
              height="100"
              preserveAspectRatio="none"
              aria-hidden="true"
            >
              <line
                x1="0"
                y1="80"
                x2="420"
                y2="80"
                stroke="var(--hairline)"
                strokeWidth="1"
                strokeDasharray="2 4"
              />
              <path
                d="M0,72 L20,68 L42,75 L66,60 L92,64 L120,52 L150,58 L180,46 L210,50 L242,38 L272,44 L302,32 L334,36 L362,28 L392,30 L420,22 L420,110 L0,110 Z"
                fill="var(--accent)"
                fillOpacity="0.10"
              />
              <path
                d="M0,72 L20,68 L42,75 L66,60 L92,64 L120,52 L150,58 L180,46 L210,50 L242,38 L272,44 L302,32 L334,36 L362,28 L392,30 L420,22"
                fill="none"
                stroke="var(--accent)"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
              <circle cx="420" cy="22" r="3" fill="var(--accent)" />
            </svg>
            <div className="chart-axis">
              <span>09:30</span>
              <span>11:00</span>
              <span>12:30</span>
              <span>14:00</span>
              <span>NOW</span>
            </div>
            <div className="ranges">
              {['1H', '1D', '1W', '1M', '3M', '1Y', 'ALL'].map((label) => (
                <button
                  key={label}
                  type="button"
                  className={`range-btn${label === '1D' ? ' active' : ''}`}
                  aria-pressed={label === '1D'}
                  disabled
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
        </aside>
      </section>
    </div>
  );
};

export default Home;
