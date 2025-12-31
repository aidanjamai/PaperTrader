import React, { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { apiRequest } from '../../services/api';
import { useAuth } from '../../hooks/useAuth';

const VerifyEmail: React.FC = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { refreshUser, isAuthenticated } = useAuth();
  const [status, setStatus] = useState<'verifying' | 'success' | 'error'>('verifying');
  const [message, setMessage] = useState('');

  useEffect(() => {
    const token = searchParams.get('token');
    if (!token) {
      setStatus('error');
      setMessage('No verification token provided');
      return;
    }

    apiRequest(`/account/verify-email?token=${token}`, { method: 'GET' })
      .then(res => res.json())
      .then(async (data) => {
        if (data.success) {
          setStatus('success');
          setMessage('Email verified successfully! Refreshing your account...');
          
          // Force refresh user profile to get updated email_verified status
          // This bypasses cache to ensure we get the latest data
          if (isAuthenticated) {
            await refreshUser();
            setMessage('Email verified successfully! Redirecting to dashboard...');
            setTimeout(() => navigate('/dashboard'), 1500);
          } else {
            setTimeout(() => navigate('/login'), 2000);
          }
        } else {
          setStatus('error');
          setMessage(data.message || 'Verification failed');
        }
      })
      .catch(() => {
        setStatus('error');
        setMessage('Failed to verify email. Please try again.');
      });
  }, [searchParams, navigate, refreshUser, isAuthenticated]);

  return (
    <div style={{ marginTop: '60px' }}>
      <div className="card">
        <h2>Email Verification</h2>
        {status === 'verifying' && <p>Verifying your email...</p>}
        {status === 'success' && (
          <div className="alert alert-success">{message}</div>
        )}
        {status === 'error' && (
          <div className="alert alert-error">{message}</div>
        )}
      </div>
    </div>
  );
};

export default VerifyEmail;

