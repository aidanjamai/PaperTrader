import React, { useState } from 'react';

function CompoundInterest() {
    const [startingBalance, setStartingBalance] = useState('');
    const [monthlyContribution, setMonthlyContribution] = useState('');
    const [years, setYears] = useState('');
    const [interestRate, setInterestRate] = useState('');
    const [result, setResult] = useState(null);

    const calculateCompoundInterest = () => {
        // Convert inputs to numbers
        const P = parseFloat(startingBalance) || 0;  // Initial principal
        const PMT = parseFloat(monthlyContribution) || 0;  // Monthly contribution
        const t = parseFloat(years) || 0;  // Time in years
        const r = parseFloat(interestRate) / 100 || 0;  // Annual interest rate (convert % to decimal)
        const n = 12;  // Compound monthly

        // Formula: A = P(1+r/n)^(nt) + PMT[((1+r/n)^(nt)-1)/(r/n)]
        const compoundFactor = Math.pow(1 + r / n, n * t);
        
        // Future value from initial principal
        const principalWithInterest = P * compoundFactor;
        
        // Future value from monthly contributions
        let contributionsWithInterest = 0;
        if (r > 0) {
            contributionsWithInterest = PMT * ((compoundFactor - 1) / (r / n));
        } else {
            // If interest rate is 0, just sum the contributions
            contributionsWithInterest = PMT * n * t;
        }
        
        const futureValue = principalWithInterest + contributionsWithInterest;
        const totalContributions = P + (PMT * n * t);
        const interestEarned = futureValue - totalContributions;

        setResult({
            futureValue: futureValue,
            totalContributions: totalContributions,
            interestEarned: interestEarned
        });
    };

    return (
        <div style={{ marginTop: '60px' }}>
            <div className="card">
                <h2>Compound Interest Calculator</h2>
                <p style={{ color: '#666', marginBottom: '24px' }}>
                    Calculate the future value of your investment with monthly contributions
                </p>

                <div className="form-group">
                    <label htmlFor="startingBalance">Initial Investment ($)</label>
                    <input
                        type="number"
                        id="startingBalance"
                        className="form-control"
                        value={startingBalance}
                        onChange={(e) => setStartingBalance(e.target.value)}
                        placeholder="e.g., 1000"
                        step="0.01"
                    />
                </div>

                <div className="form-group">
                    <label htmlFor="monthlyContribution">Monthly Contribution ($)</label>
                    <input
                        type="number"
                        id="monthlyContribution"
                        className="form-control"
                        value={monthlyContribution}
                        onChange={(e) => setMonthlyContribution(e.target.value)}
                        placeholder="e.g., 100"
                        step="0.01"
                    />
                </div>

                <div className="form-group">
                    <label htmlFor="years">Time Period (Years)</label>
                    <input
                        type="number"
                        id="years"
                        className="form-control"
                        value={years}
                        onChange={(e) => setYears(e.target.value)}
                        placeholder="e.g., 10"
                        step="0.1"
                    />
                </div>

                <div className="form-group">
                    <label htmlFor="interestRate">Annual Interest Rate (%)</label>
                    <input
                        type="number"
                        id="interestRate"
                        className="form-control"
                        value={interestRate}
                        onChange={(e) => setInterestRate(e.target.value)}
                        placeholder="e.g., 7"
                        step="0.01"
                    />
                </div>

                <button
                    onClick={calculateCompoundInterest}
                    className="btn btn-primary"
                    style={{ width: '100%' }}
                >
                    Calculate
                </button>

                {result && (
                    <div style={{ marginTop: '32px', padding: '20px', background: '#f8f9fa', borderRadius: '8px' }}>
                        <h3 style={{ color: '#667eea', marginBottom: '16px' }}>Results</h3>
                        
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid #ddd' }}>
                                <span style={{ fontWeight: 'bold', color: '#333' }}>Future Value:</span>
                                <span style={{ fontSize: '18px', color: '#667eea', fontWeight: 'bold' }}>
                                    ${result.futureValue.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                                </span>
                            </div>
                            
                            <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid #ddd' }}>
                                <span style={{ color: '#666' }}>Total Contributions:</span>
                                <span style={{ fontWeight: '500' }}>
                                    ${result.totalContributions.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                                </span>
                            </div>
                            
                            <div style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 0' }}>
                                <span style={{ color: '#666' }}>Interest Earned:</span>
                                <span style={{ fontWeight: '500', color: result.interestEarned >= 0 ? '#28a745' : '#dc3545' }}>
                                    ${result.interestEarned.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                                </span>
                            </div>
                        </div>

                        <div style={{ marginTop: '16px', padding: '12px', background: '#fff', borderRadius: '4px', fontSize: '12px', color: '#666' }}>
                            <strong>Formula used:</strong> A = P(1+r/n)^(nt) + PMT[((1+r/n)^(nt)-1)/(r/n)]
                            <br />
                            <small>Where n = 12 (monthly compounding)</small>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}

export default CompoundInterest;