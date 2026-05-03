import React from 'react';

interface PillProps {
  tone: 'gain' | 'loss';
  children: React.ReactNode;
  className?: string;
}

const Pill: React.FC<PillProps> = ({ tone, children, className = '' }) => (
  <span className={`pill pill-${tone} ${className}`.trim()}>{children}</span>
);

export default Pill;
