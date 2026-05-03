import React, { FormEvent, useEffect, useRef, useState } from 'react';
import { useWatchlist } from '../../hooks/useWatchlist';
import { formatMoney, formatPercent, formatSignedMoney } from '../primitives/format';

const SYMBOL_PATTERN = /^[A-Z]{1,10}(\.[A-Z]{1,2})?$/;

interface AddWatchlistModalProps {
  isOpen: boolean;
  onClose: () => void;
  onAdd: (symbol: string) => Promise<void>;
}

const AddWatchlistModal: React.FC<AddWatchlistModalProps> = ({ isOpen, onClose, onAdd }) => {
  const [input, setInput] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      setInput('');
      setError(null);
      setSubmitting(false);
      // Defer focus until after the modal is painted.
      const id = window.setTimeout(() => inputRef.current?.focus(), 0);
      return () => window.clearTimeout(id);
    }
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const symbol = input.trim().toUpperCase();
    if (!symbol) return;
    if (!SYMBOL_PATTERN.test(symbol)) {
      setError('Use 1–10 letters (e.g., AAPL, BRK.B).');
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      await onAdd(symbol);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add symbol');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <div>
            <div className="eyebrow" style={{ marginBottom: 4 }}>
              Watchlist
            </div>
            <h2>Add a symbol</h2>
          </div>
          <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>

        <div className="modal-body">
          {error && <div className="alert alert-error">{error}</div>}

          <form onSubmit={handleSubmit}>
            <div className="form-group">
              <label htmlFor="watchlist-add-symbol" className="form-label">
                Symbol
              </label>
              <input
                ref={inputRef}
                id="watchlist-add-symbol"
                type="text"
                className="input mono"
                value={input}
                onChange={(e) => {
                  setInput(e.target.value.toUpperCase());
                  if (error) setError(null);
                }}
                placeholder="AAPL"
                maxLength={13}
                disabled={submitting}
                spellCheck={false}
                autoComplete="off"
                style={{ textTransform: 'uppercase' }}
              />
              <div className="form-hint">
                We'll verify the symbol against live market data before adding it.
              </div>
            </div>

            <div style={{ display: 'flex', gap: 8 }}>
              <button
                type="submit"
                className="btn btn-primary"
                style={{ flex: 1 }}
                disabled={submitting || !input.trim()}
              >
                {submitting ? 'Adding…' : 'Add to watchlist'}
              </button>
              <button
                type="button"
                className="btn btn-secondary"
                onClick={onClose}
                disabled={submitting}
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
};

const WatchlistCard: React.FC = () => {
  const { items, loading, error, fetchWatchlist, addSymbol, removeSymbol } = useWatchlist();
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [pendingRemove, setPendingRemove] = useState<string | null>(null);
  const [removeError, setRemoveError] = useState<string | null>(null);

  useEffect(() => {
    fetchWatchlist();
  }, [fetchWatchlist]);

  const handleRemove = async (symbol: string) => {
    setRemoveError(null);
    setPendingRemove(symbol);
    try {
      await removeSymbol(symbol);
    } catch (err) {
      setRemoveError(
        err instanceof Error ? err.message : `Failed to remove ${symbol}`
      );
    } finally {
      setPendingRemove((current) => (current === symbol ? null : current));
    }
  };

  return (
    <section className="panel" style={{ marginTop: 24 }}>
      <div className="panel-head">
        <h3>Watchlist</h3>
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => setIsAddOpen(true)}
          aria-label="Add symbol to watchlist"
          style={{
            padding: 0,
            width: 32,
            height: 32,
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 18,
            lineHeight: 1,
          }}
        >
          <span aria-hidden="true">+</span>
        </button>
      </div>

      {(error || removeError) && (
        <div className="alert alert-error" style={{ margin: '12px 18px 0' }}>
          {removeError ?? error}
        </div>
      )}

      {loading ? (
        <div className="empty-state">
          <span>Loading watchlist…</span>
        </div>
      ) : items.length === 0 ? (
        <div className="empty-state">
          <span className="empty-title">No symbols watched yet</span>
          <span>Tap + to start tracking a ticker without holding shares.</span>
        </div>
      ) : (
        <table className="holdings">
          <thead>
            <tr>
              <th>Symbol</th>
              <th className="num">Price</th>
              <th className="num">Change</th>
              <th className="num">% Change</th>
              <th className="num" aria-label="Actions" />
            </tr>
          </thead>
          <tbody>
            {items.map((entry) => {
              const pillTone: 'gain' | 'loss' | null = entry.has_price
                ? entry.change > 0
                  ? 'gain'
                  : entry.change < 0
                  ? 'loss'
                  : null
                : null;
              const removing = pendingRemove === entry.symbol;
              return (
                <tr key={entry.id}>
                  <td>
                    <div className="ticker">
                      <span className="mark" aria-hidden="true">
                        {entry.symbol.charAt(0)}
                      </span>
                      <span className="sym">{entry.symbol}</span>
                    </div>
                  </td>
                  <td className="num">
                    {entry.has_price ? formatMoney(entry.price) : '—'}
                  </td>
                  <td className="num">
                    {!entry.has_price
                      ? '—'
                      : pillTone
                      ? (
                        <span className={`pill pill-${pillTone}`}>
                          {formatSignedMoney(entry.change)}
                        </span>
                      )
                      : formatSignedMoney(entry.change)}
                  </td>
                  <td className="num">
                    {!entry.has_price
                      ? '—'
                      : pillTone
                      ? (
                        <span className={`pill pill-${pillTone}`}>
                          {formatPercent(entry.change_percentage)}
                        </span>
                      )
                      : formatPercent(entry.change_percentage)}
                  </td>
                  <td className="num">
                    <button
                      type="button"
                      className="btn btn-secondary"
                      onClick={() => handleRemove(entry.symbol)}
                      disabled={removing}
                      aria-label={`Remove ${entry.symbol} from watchlist`}
                      style={{ padding: '4px 10px', fontSize: 12 }}
                    >
                      {removing ? 'Removing…' : 'Remove'}
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      <AddWatchlistModal
        isOpen={isAddOpen}
        onClose={() => setIsAddOpen(false)}
        onAdd={addSymbol}
      />
    </section>
  );
};

export default WatchlistCard;
