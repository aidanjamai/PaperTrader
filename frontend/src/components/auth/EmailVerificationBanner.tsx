import React, { useState } from 'react';
import { apiRequest } from '../../services/api';

interface EmailVerificationBannerProps {
  email: string;
  onVerified?: () => void;
}

const EmailVerificationBanner: React.FC<EmailVerificationBannerProps> = ({ email, onVerified }) => {
  const [resending, setResending] = useState(false);
  const [resendMessage, setResendMessage] = useState<string>('');

  const handleResendVerification = async () => {
    setResending(true);
    setResendMessage('');
    
    try {
      const response = await apiRequest('/account/resend-verification', {
        method: 'POST',
        body: JSON.stringify({ email })
      });

      const data = await response.json();
      if (response.ok && data.success) {
        setResendMessage('Verification email sent! Please check your inbox.');
      } else {
        setResendMessage(data.message || 'Failed to resend verification email');
      }
    } catch (error) {
      setResendMessage('Failed to resend verification email. Please try again.');
    } finally {
      setResending(false);
      setTimeout(() => setResendMessage(''), 5000);
    }
  };

  return (
    <div style={{
      background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
      color: 'white',
      padding: '16px 20px',
      marginBottom: '20px',
      borderRadius: '8px',
      boxShadow: '0 4px 6px rgba(0, 0, 0, 0.1)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      flexWrap: 'wrap',
      gap: '12px'
    }}>
      <div style={{ flex: 1, minWidth: '250px' }}>
        <div style={{ 
          display: 'flex', 
          alignItems: 'center', 
          gap: '12px',
          marginBottom: resendMessage ? '8px' : '0'
        }}>
          <span style={{ fontSize: '20px' }}>⚠️</span>
          <div>
            <strong style={{ fontSize: '16px', display: 'block', marginBottom: '4px' }}>
              Please verify your email address
            </strong>
            <p style={{ margin: 0, fontSize: '14px', opacity: 0.95 }}>
              We sent a verification link to <strong>{email}</strong>. Click the link in your email to verify your account and start trading.
            </p>
          </div>
        </div>
        {resendMessage && (
          <div style={{
            marginTop: '8px',
            padding: '8px 12px',
            background: 'rgba(255, 255, 255, 0.2)',
            borderRadius: '4px',
            fontSize: '14px'
          }}>
            {resendMessage}
          </div>
        )}
      </div>
      <button
        onClick={handleResendVerification}
        disabled={resending}
        style={{
          background: 'white',
          color: '#667eea',
          border: 'none',
          padding: '10px 20px',
          borderRadius: '6px',
          cursor: resending ? 'not-allowed' : 'pointer',
          fontWeight: '600',
          fontSize: '14px',
          opacity: resending ? 0.7 : 1,
          transition: 'opacity 0.2s',
          whiteSpace: 'nowrap'
        }}
      >
        {resending ? 'Sending...' : 'Resend Email'}
      </button>
    </div>
  );
};

export default EmailVerificationBanner;

