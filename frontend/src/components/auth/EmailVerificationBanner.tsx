import React, { useState } from 'react';
import { apiRequest } from '../../services/api';

interface EmailVerificationBannerProps {
  email: string;
  onVerified?: () => void;
}

const EmailVerificationBanner: React.FC<EmailVerificationBannerProps> = ({ email }) => {
  const [resending, setResending] = useState(false);
  const [resendMessage, setResendMessage] = useState<string>('');

  const handleResendVerification = async () => {
    setResending(true);
    setResendMessage('');

    try {
      const response = await apiRequest('/account/resend-verification', {
        method: 'POST',
        body: JSON.stringify({ email }),
      });

      const data = await response.json();
      if (response.ok && data.success) {
        setResendMessage('Verification email sent — check your inbox.');
      } else {
        setResendMessage(data.message || 'Failed to resend verification email.');
      }
    } catch {
      setResendMessage('Failed to resend verification email. Please try again.');
    } finally {
      setResending(false);
      setTimeout(() => setResendMessage(''), 5000);
    }
  };

  return (
    <div
      className="verify-banner"
      role="status"
      style={{
        background: 'var(--accent-tint)',
        border: '1px solid var(--hairline)',
        color: 'var(--ink)',
        padding: '14px 18px',
        marginBottom: 20,
        marginTop: 16,
        borderRadius: 8,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        flexWrap: 'wrap',
        gap: 12,
      }}
    >
      <div style={{ flex: 1, minWidth: 240 }}>
        <div className="eyebrow" style={{ marginBottom: 6, color: 'var(--accent)' }}>
          Action required
        </div>
        <div style={{ fontSize: 14, lineHeight: 1.5 }}>
          Verify <span className="mono" style={{ color: 'var(--ink)' }}>{email}</span> to start
          trading. We sent you a link — click it to activate your account.
        </div>
        {resendMessage && (
          <div className="mono" style={{ marginTop: 8, fontSize: 12, color: 'var(--ink-muted)' }}>
            {resendMessage}
          </div>
        )}
      </div>
      <button
        type="button"
        className="btn btn-secondary btn-sm"
        onClick={handleResendVerification}
        disabled={resending}
      >
        {resending ? 'Sending…' : 'Resend email'}
      </button>
    </div>
  );
};

export default EmailVerificationBanner;
