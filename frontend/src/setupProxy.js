const { createProxyMiddleware } = require('http-proxy-middleware');

/**
 * Development proxy configuration
 * 
 * Proxies API requests to the backend server
 */
module.exports = function(app) {
  // Use BACKEND_URL env var, or default to localhost for local development
  // 'http://backend:8080' is for Docker Compose networking
  const backendUrl = process.env.BACKEND_URL || 'http://localhost:8080';
  
  console.log('[Proxy] Configuring proxy for /api ->', backendUrl);
  
  app.use(
    '/api',
    createProxyMiddleware({
      target: backendUrl,
      changeOrigin: true,
      logLevel: 'debug',
      secure: false,
      // Don't follow redirects - return them to the client
      followRedirects: false,
      // Preserve the path - forward /api/investments to http://localhost:8080/api/investments
      pathRewrite: {
        '^/api': '/api', // Keep /api in the path
      },
      onError: (err, req, res) => {
        console.error('[Proxy] Error proxying request:', err.message);
        console.error('[Proxy] Request URL:', req.url);
        console.error('[Proxy] Target:', backendUrl);
        res.status(500).json({ error: 'Proxy error: ' + err.message });
      },
      onProxyReq: (proxyReq, req, res) => {
        console.log('[Proxy] Proxying', req.method, req.url, '->', backendUrl + req.url);
      },
      onProxyRes: (proxyRes, req, res) => {
        console.log('[Proxy] Response status:', proxyRes.statusCode, 'for', req.url);
        // Log redirects
        if (proxyRes.statusCode >= 300 && proxyRes.statusCode < 400) {
          const location = proxyRes.headers['location'];
          console.log('[Proxy] Redirect to:', location);
        }
      },
    })
  );
};


