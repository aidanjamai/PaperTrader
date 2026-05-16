import React, { useState } from 'react';
import { ResearchAnswer, ResearchCitation } from '../../types/api';
import { askResearch, ApiError } from '../../services/api';

// ---- Refusal copy map ----
interface RefusalCopy {
  title: string;
  body: string;
}

function getRefusalCopy(reason: string | undefined): RefusalCopy {
  switch (reason) {
    case 'forward_looking':
      return {
        title: "I can't make predictions",
        body: "I don't give buy/sell recommendations or price forecasts. Try asking what a company has already disclosed — for example: 'What did NVIDIA say about export controls in their last 10-Q?'",
      };
    case 'no_sources':
      return {
        title: 'Nothing in my sources matches',
        body: "I couldn't find anything in my SEC-filing corpus that supports an answer to that question.",
      };
    case 'out_of_scope':
      return {
        title: "That's outside what I can help with",
        body: 'I only answer questions about company financials and disclosures from the documents I have.',
      };
    case 'hallucinated_citation':
      return {
        title: "I couldn't verify that answer",
        body: "The model returned an answer but cited sources I couldn't validate. This is rare — try rephrasing the question.",
      };
    case 'model_error':
      return {
        title: 'The model returned an unexpected response',
        body: 'Something went wrong with the response format. Try again.',
      };
    default:
      return {
        title: "I couldn't answer that",
        body: 'Try rephrasing your question.',
      };
  }
}

// ---- Inline [N] marker renderer ----
function renderAnswerWithCitations(
  text: string,
  citationCount: number
): React.ReactNode[] {
  // Split on [N] patterns, keeping the delimiters
  const parts = text.split(/(\[\d+\])/g);
  return parts.map((part, idx) => {
    const match = part.match(/^\[(\d+)\]$/);
    if (match) {
      const n = parseInt(match[1], 10);
      if (n >= 1 && n <= citationCount) {
        return (
          <sup key={idx} className="cite-marker">
            <a href={`#cite-${n}`}>{n}</a>
          </sup>
        );
      }
      // N is out of range — render as plain text
      return <React.Fragment key={idx}>{part}</React.Fragment>;
    }
    return <React.Fragment key={idx}>{part}</React.Fragment>;
  });
}

// ---- Citation item ----
const CitationItem: React.FC<{ citation: ResearchCitation; index: number }> = ({
  citation,
  index,
}) => {
  const n = index + 1;
  const filedDate = citation.filed_at ? citation.filed_at.slice(0, 10) : null;
  const displayUrl =
    citation.source_url.length > 60
      ? citation.source_url.slice(0, 57) + '…'
      : citation.source_url;

  return (
    <div id={`cite-${n}`} className="research-source">
      <div className="research-source-meta">
        <span className="research-source-num">[{n}]</span>
        {citation.symbol && (
          <span className="eyebrow" style={{ marginLeft: 8 }}>
            {citation.symbol}
          </span>
        )}
        {filedDate && (
          <span className="muted" style={{ marginLeft: 8, fontSize: 12 }}>
            {filedDate}
          </span>
        )}
        <span className="muted" style={{ marginLeft: 8, fontSize: 12 }}>
          {(citation.score * 100).toFixed(1)}% similarity
        </span>
      </div>
      <div style={{ marginTop: 4 }}>
        <a
          href={citation.source_url}
          target="_blank"
          rel="noopener noreferrer"
          className="research-source-url"
        >
          {displayUrl}
        </a>
      </div>
      <blockquote className="research-quote">{citation.excerpt}</blockquote>
    </div>
  );
};

// ---- Refusal card ----
const RefusalCard: React.FC<{
  answer: ResearchAnswer;
  onPick: (q: string) => void;
}> = ({ answer, onPick }) => {
  const copy = getRefusalCopy(answer.refusal_reason);
  const coverage =
    answer.refusal_reason === 'no_sources' ? answer.coverage : undefined;
  const hasCoverage = !!coverage && coverage.symbols.length > 0;

  return (
    <div className="panel" style={{ marginTop: 24 }}>
      <div style={{ padding: '20px 24px' }}>
        <h4 style={{ margin: '0 0 8px 0', color: 'var(--ink)' }}>{copy.title}</h4>
        <p
          style={{
            margin: hasCoverage ? '0 0 16px 0' : '0 0 16px 0',
            color: 'var(--ink-muted)',
            lineHeight: 1.6,
          }}
        >
          {copy.body}
        </p>

        {hasCoverage && (
          <>
            <p className="muted" style={{ margin: '0 0 8px 0', fontSize: 12 }}>
              I have filings for these tickers — click one:
            </p>
            <div
              style={{
                display: 'flex',
                flexWrap: 'wrap',
                gap: 8,
                marginBottom: 16,
              }}
            >
              {coverage!.symbols.map((sym) => (
                <button
                  key={sym}
                  type="button"
                  className="btn btn-secondary"
                  style={{ fontSize: 12, padding: '4px 10px' }}
                  onClick={() =>
                    onPick(
                      `What risk factors did ${sym} disclose in its latest 10-K?`
                    )
                  }
                >
                  {sym}
                </button>
              ))}
            </div>

            {coverage!.examples && coverage!.examples.length > 0 && (
              <>
                <p
                  className="muted"
                  style={{ margin: '0 0 8px 0', fontSize: 12 }}
                >
                  Or try one of these:
                </p>
                <div
                  style={{
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 8,
                    marginBottom: 16,
                  }}
                >
                  {coverage!.examples.map((ex) => (
                    <button
                      key={ex}
                      type="button"
                      className="btn btn-ghost"
                      style={{ textAlign: 'left', fontSize: 13 }}
                      onClick={() => onPick(ex)}
                    >
                      {ex}
                    </button>
                  ))}
                </div>
              </>
            )}
          </>
        )}

        <p className="muted" style={{ margin: 0, fontSize: 12 }}>
          Took {answer.latency_ms}ms
        </p>
      </div>
    </div>
  );
};

// ---- Answer card ----
const AnswerCard: React.FC<{ answer: ResearchAnswer }> = ({ answer }) => {
  return (
    <div className="panel" style={{ marginTop: 24 }}>
      <div style={{ padding: '20px 24px' }}>
        <div
          style={{
            whiteSpace: 'pre-wrap',
            lineHeight: 1.7,
            color: 'var(--ink)',
            marginBottom: 12,
          }}
        >
          {renderAnswerWithCitations(answer.answer, answer.citations.length)}
        </div>
        <p className="muted" style={{ margin: '0 0 0 0', fontSize: 12, borderTop: '1px solid var(--hairline)', paddingTop: 12 }}>
          Latency: {answer.latency_ms}ms &middot; Sources: {answer.citations.length}
        </p>
      </div>

      {answer.citations.length > 0 && (
        <div style={{ borderTop: '1px solid var(--hairline)', padding: '20px 24px' }}>
          <h4 style={{ margin: '0 0 16px 0', fontSize: 14, fontWeight: 600, color: 'var(--ink)' }}>
            Sources
          </h4>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
            {answer.citations.map((c, i) => (
              <CitationItem key={c.chunk_id} citation={c} index={i} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

// ---- Main page ----
const Research: React.FC = () => {
  const [query, setQuery] = useState<string>('');
  const [symbolsInput, setSymbolsInput] = useState<string>('');
  const [answer, setAnswer] = useState<ResearchAnswer | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const runQuery = async (rawQuery: string) => {
    const trimmedQuery = rawQuery.trim();

    if (!trimmedQuery) return;
    if (trimmedQuery.length > 2000) {
      setError('Query must be 2000 characters or fewer.');
      return;
    }

    const parsedSymbols = symbolsInput
      .split(',')
      .map((s) => s.trim().toUpperCase())
      .filter(Boolean);

    setLoading(true);
    setError(null);
    setAnswer(null);

    try {
      const result = await askResearch(
        trimmedQuery,
        parsedSymbols.length > 0 ? parsedSymbols : undefined
      );

      // Defensive: empty answer text with refused=false → treat as model_error refusal
      if (!result.refused && !result.answer?.trim()) {
        setAnswer({ ...result, refused: true, refusal_reason: 'model_error' });
      } else {
        setAnswer(result);
      }
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 429) {
          setError("You've hit the limit of 10 questions per minute. Try again in a moment.");
        } else if (err.status === 401) {
          setError('You are not logged in. Please log in and try again.');
        } else {
          setError('Something went wrong. Please try again.');
        }
      } else {
        setError('Something went wrong. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    runQuery(query);
  };

  // Clicking a coverage chip / example in the refusal card: fill the box so
  // the user sees what was asked, then run it.
  const handlePick = (q: string) => {
    setQuery(q);
    runQuery(q);
  };

  const isSubmitDisabled = loading || !query.trim();

  return (
    <div className="container">
      <header className="page-header">
        <div>
          <div className="eyebrow" style={{ marginBottom: 6 }}>
            Universe &middot; financial research
          </div>
          <h1>Research</h1>
        </div>
      </header>

      <section className="panel" style={{ marginBottom: 24 }}>
        <div className="panel-head">
          <div>
            <h3 style={{ margin: 0 }}>Ask a question</h3>
            <p className="muted" style={{ margin: '4px 0 0 0', fontSize: 12 }}>
              Answers from SEC filings (10-K, 10-Q, 8-K) and market news.
            </p>
          </div>
        </div>
        <form onSubmit={handleSubmit} style={{ padding: '20px 24px' }}>
          <div className="form-group">
            <label htmlFor="research-query" className="form-label">
              Question
            </label>
            <textarea
              id="research-query"
              className="input"
              rows={5}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="e.g. What did Apple disclose as supply chain risks in their most recent 10-K?"
              disabled={loading}
              maxLength={2000}
            />
            <div className="form-hint">{query.length}/2000 characters</div>
          </div>

          <div className="form-group">
            <label htmlFor="research-symbols" className="form-label">
              Ticker filter
              <span className="muted" style={{ fontWeight: 400, marginLeft: 6 }}>
                (optional)
              </span>
            </label>
            <input
              id="research-symbols"
              type="text"
              className="input"
              value={symbolsInput}
              onChange={(e) => setSymbolsInput(e.target.value)}
              placeholder="Filter by ticker (optional, comma-separated): AAPL, MSFT"
              disabled={loading}
            />
            <div className="form-hint">
              Leave blank to search all available sources. Provide tickers like AAPL or MSFT to
              narrow results.
            </div>
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            disabled={isSubmitDisabled}
          >
            {loading ? 'Asking…' : 'Ask'}
          </button>
        </form>
      </section>

      {error && <div className="alert alert-error">{error}</div>}

      {!error && loading && (
        <div className="empty-state">Asking your question…</div>
      )}

      {!loading && !error && answer && answer.refused && (
        <RefusalCard answer={answer} onPick={handlePick} />
      )}

      {!loading && !error && answer && !answer.refused && (
        <AnswerCard answer={answer} />
      )}

      {!loading && !error && !answer && (
        <div className="empty-state">
          <span className="empty-title">No results yet</span>
          <span>Type a question above and press Ask.</span>
        </div>
      )}

      <p
        className="muted"
        style={{ marginTop: 40, fontSize: 12, lineHeight: 1.6, textAlign: 'center' }}
      >
        Queries are processed by Voyage AI (embeddings) and Groq (generation) on free tiers.
        Sources are limited to recent SEC filings (10-K / 10-Q / 8-K) and market news.
      </p>
    </div>
  );
};

export default Research;
