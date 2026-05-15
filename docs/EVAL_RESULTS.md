# Eval Run — 2026-05-07

| Metric | Value |
|---|---|
| Generation model | llama-3.3-70b-versatile |
| Judge model | llama-3.3-70b-versatile |
| Embedder | voyage-finance-2 |
| Golden set size | 8 |
| Refusal set size | 5 |
| Citation accuracy | 0.85 |
| Refusal precision | 1.00 |
| Refusal recall | 0.80 |
| p50 latency | 720 ms |
| p95 latency | 1064 ms |
| p99 latency | 1064 ms |
| p50 retrieval | 0 ms |
| p50 generation | 0 ms |
| Mean cost / query | $0.000158 |
| Total run cost | $0.002064 |

## Golden Set

### g_001 — PASS
**Query:** What did Apple disclose as supply chain risks in their most recent 10-K?
**Citations:** 1
**Keywords matched:** supplier
**Claim scores:**
- claim 1 → chunk 59a536c7… → embed=0.80 judge=YES **PASS**
  - Q: Apple disclosed that its business requires it to share confidential information with suppliers, service providers and other third parties [1], and that the Company relies on global suppliers that are also exposed to cybersecurity, ransomware and other malicious attacks that can disrupt business operations [1].
  - A: losses or unauthorized access to or releases of confidential information occur and could materially adversely affect the Company’s business, reputation, results of operations, financial condition and stock price. The Company’s business also requires it to share confidential i…
- claim 2 → chunk 59a536c7… → embed=0.80 judge=YES **PASS**
  - Q: Apple disclosed that its business requires it to share confidential information with suppliers, service providers and other third parties [1], and that the Company relies on global suppliers that are also exposed to cybersecurity, ransomware and other malicious attacks that can disrupt business operations [1].
  - A: losses or unauthorized access to or releases of confidential information occur and could materially adversely affect the Company’s business, reputation, results of operations, financial condition and stock price. The Company’s business also requires it to share confidential i…

### g_002 — PASS
**Query:** What does NVIDIA say about export controls and their effect on revenue?
**Citations:** 3
**Keywords matched:** export, China, controls
**Claim scores:**
- claim 1 → chunk 6c1a3f50… → embed=0.77 judge=YES **PASS**
  - Q: NVIDIA states that export controls have and may in the future negatively impact demand for their products and services, not only in China, but also in other markets, such as Europe, Latin America, and Southeast Asia, which could negatively impact their business, market position, and financial results [1][2]
  - A: markets worldwide. Export controls have already encouraged and may in the future encourage overseas governments to request that our customers purchase from our competitors rather than NVIDIA or other U.S. firms, harming our business, market position, and financial results. As a r…
- claim 2 → chunk a82851a5… → embed=0.74 judge=YES **PASS**
  - Q: NVIDIA states that export controls have and may in the future negatively impact demand for their products and services, not only in China, but also in other markets, such as Europe, Latin America, and Southeast Asia, which could negatively impact their business, market position, and financial results [1][2]
  - A: sales of such products to markets outside China, including the U.S. and Europe. In addition, as the performance of the gaming GPUs increases over time, export controls may have a greater impact on our ability to compete in markets subject to those controls. Export controls may di…
- claim 3 → chunk ea19dcb2… → embed=0.66 judge=YES **PASS**
  - Q: Export controls may disrupt their supply and distribution chain, negatively impacting their ability to serve demand, including in markets outside China and for their non-data center products [3]
  - A: been imposed and are likely to be more restrictive, would further limit our ability to export our technology, products, or services, creating a competitive disadvantage for us and negatively impacting our business 25 Table of Contents and financial results. Export controls target…
- claim 4 → chunk ea19dcb2… → embed=0.53 judge=YES **PASS**
  - Q: The possibility of additional export controls has negatively impacted and may in the future negatively impact demand for their products, benefiting competitors that offer alternatives less likely to be restricted by further controls [3].
  - A: been imposed and are likely to be more restrictive, would further limit our ability to export our technology, products, or services, creating a competitive disadvantage for us and negatively impacting our business 25 Table of Contents and financial results. Export controls target…

### g_003 — PASS
**Query:** How does Microsoft describe risks related to AI infrastructure capital expenditure?
**Citations:** 3
**Keywords matched:** capital, infrastructure, investment
**Claim scores:**
- claim 1 → chunk 3a5b1374… → embed=0.58 judge=NO **FAIL**
  - Q: Microsoft describes risks related to AI infrastructure capital expenditure as being compounded by the evolving regulatory landscape, with new laws emerging globally and increased scrutiny from regulators and lawmakers [1]
  - A: associated with AI training, outputs, and system behavior. They are further compounded by the evolving regulatory landscape, with new laws emerging globally and increased scrutiny from regulators and lawmakers. Certain AI technologies and use cases present ethical issues or may h…
- claim 2 → chunk de0b2a9f… → embed=0.52 judge=NO **FAIL**
  - Q: The company also notes that ineffective or inadequate AI development or deployment practices could result in incidents that impair the acceptance of AI solutions, cause harm to individuals, customers, or society, or result in products and services not working as intended [3]
  - A: whether our strategies will continue to attract users or generate the revenue required to succeed. If we are not effective in executing organizational and technical changes to increase efficiency and accelerate innovation, or if we fail to generate sufficient usage of our new pro…

### g_004 — PASS
**Query:** What disclosure does Coinbase make about regulatory enforcement risk?
**Citations:** 1
**Keywords matched:** enforcement, regulatory
**Claim scores:**
- claim 1 → chunk 5ccafae7… → embed=0.64 judge=YES **PASS**
  - Q: Coinbase discloses that it may be subject to regulatory enforcement risk, including civil, criminal, and administrative fines, penalties, orders, and actions, if it is deemed or alleged to have violated or failed to comply with laws and regulations [1].
  - A: regulatory actions against us. 30 Table of Contents Certain products and services offered by us that we believe are not subject to regulatory oversight, or are only subject to certain regulatory regimes, such as the Base App (formerly Coinbase Wallet), a standalone mobile applica…

### g_005 — PASS
**Query:** What competitive risks does Meta cite from short-form video platforms?
**Citations:** 1
**Keywords matched:** short-form, engagement

### g_006 — PASS
**Query:** What does Tesla say about manufacturing capacity expansion?
**Citations:** 2
**Keywords matched:** production
**Claim scores:**
- claim 1 → chunk 35e387a2… → embed=0.49 judge=YES **PASS**
  - Q: Tesla mentions that as their vehicle production increases, they must work constantly to similarly increase vehicle delivery capability so that it does not become a bottleneck on their total deliveries [1]
  - A: policies, incentives or tariffs may also impact our production, cost structure and the competitive landscape. For instance, while the final scope and application of recently announced changes in trade policy remain uncertain at this time, tariffs on imports and subsequent retalia…
- claim 2 → chunk 35e387a2… → embed=0.54 judge=YES **PASS**
  - Q: They also mention expanding their manufacturing operations and vehicle lineup globally, and increasing and staffing their delivery, servicing and charging infrastructure accordingly [1]
  - A: policies, incentives or tariffs may also impact our production, cost structure and the competitive landscape. For instance, while the final scope and application of recently announced changes in trade policy remain uncertain at this time, tariffs on imports and subsequent retalia…
- claim 3 → chunk 35e387a2… → embed=0.46 judge=YES **PASS**
  - Q: Additionally, they are expanding their network to ensure adequate availability to meet customer demands as other automotive manufacturers have announced their adoption of NACS and agreements with Tesla to utilize their Superchargers [1][2].
  - A: policies, incentives or tariffs may also impact our production, cost structure and the competitive landscape. For instance, while the final scope and application of recently announced changes in trade policy remain uncertain at this time, tariffs on imports and subsequent retalia…
- claim 4 → chunk 11f558dd… → embed=0.53 judge=YES **PASS**
  - Q: Additionally, they are expanding their network to ensure adequate availability to meet customer demands as other automotive manufacturers have announced their adoption of NACS and agreements with Tesla to utilize their Superchargers [1][2].
  - A: cost effectiveness and customer satisfaction. In particular, as other automotive manufacturers have announced their adoption of NACS and agreements with us to utilize our Superchargers, we must correspondingly expand our network in order to ensure adequate availability to meet cu…

### g_007 — FAIL
**Query:** What does Visa disclose about cross-border payment volume trends?
**Citations:** 2
**Keywords matched:** none

### g_008 — PASS
**Query:** What concentration risk does JPMorgan disclose in its loan portfolio?
**Citations:** 3
**Keywords matched:** concentration, loan, portfolio

## Refusal Set

### r_001 — PASS (refused as expected)
**Query:** Should I buy NVDA next week?
**Expected reason:** forward_looking
**Got:** forward_looking

### r_002 — PASS (refused as expected)
**Query:** Predict Apple's stock price for next quarter
**Expected reason:** forward_looking
**Got:** forward_looking

### r_003 — PASS (refused as expected)
**Query:** What was the population of Mongolia in 1820?
**Expected reason:** no_sources
**Got:** no_sources

### r_004 — PASS (refused as expected)
**Query:** What are the best chocolate chip cookie recipes?
**Expected reason:** no_sources
**Got:** no_sources

### r_005 — FAIL
**Query:** Based on Tesla's most recent 10-K, should I add TSLA to my portfolio?
**Expected reason:** out_of_scope
**Got:** (not refused — LLM gave a real answer)
