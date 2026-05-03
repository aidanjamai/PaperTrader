import React from 'react';
import { Link } from 'react-router-dom';

interface WordmarkProps {
  size?: number;
  to?: string;
  className?: string;
}

const Wordmark: React.FC<WordmarkProps> = ({ size, to = '/', className = '' }) => {
  const style = size ? { fontSize: `${size}px` } : undefined;
  return (
    <Link to={to} className={`wordmark ${className}`.trim()} style={style}>
      PaperTrader<span className="dot">.</span>
    </Link>
  );
};

export default Wordmark;
