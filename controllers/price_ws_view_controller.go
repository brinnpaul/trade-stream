package controllers

import "net/http"

type PriceWsViewController struct {
}

func NewPriceWsViewController() *PriceWsViewController {
	return &PriceWsViewController{}
}

// ServiceTickerPage serves the HTML page for real-time price display
func (tc *PriceWsViewController) ServiceTickerPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/prices" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Live Price Stream</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .price-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
        }
        .price-card {
            background: white;
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            border-left: 4px solid #007bff;
            transition: transform 0.2s ease, box-shadow 0.2s ease;
        }
        .price-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 8px rgba(0,0,0,0.15);
        }
        .price-card.updated {
            animation: highlight 0.5s ease;
        }
        @keyframes highlight {
            0% { background-color: #fff3cd; }
            100% { background-color: white; }
        }
        .symbol {
            font-size: 24px;
            font-weight: bold;
            color: #333;
            margin-bottom: 10px;
        }
        .price {
            font-size: 32px;
            font-weight: bold;
            color: #007bff;
            margin-bottom: 10px;
        }
        .timestamp {
            font-size: 12px;
            color: #666;
        }
        .status {
            text-align: center;
            padding: 10px;
            margin-bottom: 20px;
            border-radius: 4px;
        }
        .connected {
            background-color: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        .disconnected {
            background-color: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        .loading {
            background-color: #fff3cd;
            color: #856404;
            border: 1px solid #ffeaa7;
        }
        .stats {
            text-align: center;
            margin-bottom: 20px;
            font-size: 14px;
            color: #666;
        }
        .no-Data {
            text-align: center;
            padding: 40px;
            color: #666;
            font-style: italic;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Live Price Stream</h1>
        <div id="status" class="status loading">Connecting...</div>
        <div id="stats" class="stats">No tickers available</div>
    </div>
    
    <div id="priceGrid" class="price-grid">
    </div>

    <script>
        let ws;
        let reconnectAttempts = 0;
        const maxReconnectAttempts = 5;
        let currentPrices = {};
        let priceElements = {};

        function connect() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws/prices';
            
            ws = new WebSocket(wsUrl);
            
            ws.onopen = function() {
                updateStatus('Connected', 'connected');
                reconnectAttempts = 0;
            };
            
            ws.onmessage = function(event) {
                const prices = JSON.parse(event.data);
                updatePrices(prices);
            };
            
            ws.onclose = function() {
                updateStatus('Disconnected', 'disconnected');
                if (reconnectAttempts < maxReconnectAttempts) {
                    reconnectAttempts++;
                    setTimeout(connect, 2000 * reconnectAttempts);
                }
            };
            
            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
                updateStatus('Error', 'disconnected');
            };
        }

        function updateStatus(message, className) {
            const statusEl = document.getElementById('status');
            statusEl.textContent = message;
            statusEl.className = 'status ' + className;
        }

        function updateStats(prices) {
            const statsEl = document.getElementById('stats');
            const tickerCount = Object.keys(prices).length;
            const lastUpdate = new Date().toLocaleTimeString();
            statsEl.textContent = tickerCount + ' ticker' + (tickerCount !== 1 ? 's' : '') + ' • Last update: ' + lastUpdate;
        }

        function createPriceCard(symbol, ticker) {
            const card = document.createElement('div');
            card.className = 'price-card';
            card.id = 'card-' + symbol;
            
            card.innerHTML = 
                '<div class="symbol">' + symbol + '</div>' +
                '<div class="price" id="' + symbol + '-price">' + (ticker.Price || '--') + '</div>' +
                '<div class="timestamp" id="' + symbol + '-time">' + (ticker.lastUpdatedTimeUtc ? new Date(ticker.lastUpdatedTimeUtc).toLocaleString() : '--') + '</div>';
            
            return card;
        }

        function updatePrices(prices) {
            const priceGrid = document.getElementById('priceGrid');
            const newSymbols = Object.keys(prices);
            
            // Clear the grid if no Data
            if (newSymbols.length === 0) {
                priceGrid.innerHTML = '<div class="no-Data">No ticker Data available</div>';
                return;
            }
            
            // Remove old cards for symbols that no longer exist
            Object.keys(priceElements).forEach(symbol => {
                if (!newSymbols.includes(symbol)) {
                    const card = document.getElementById('card-' + symbol);
                    if (card) {
                        card.remove();
                    }
                    delete priceElements[symbol];
                }
            });
            
            // Add new cards or update existing ones
            newSymbols.forEach(symbol => {
                const ticker = prices[symbol];
                
                if (!priceElements[symbol]) {
                    // Create new card
                    const card = createPriceCard(symbol, ticker);
                    priceGrid.appendChild(card);
                    priceElements[symbol] = {
                        priceEl: document.getElementById(symbol + '-price'),
                        timeEl: document.getElementById(symbol + '-time')
                    };
                } else {
                    // Update existing card
                    const { priceEl, timeEl } = priceElements[symbol];
                    
                    if (priceEl && timeEl) {
                        // Check if price changed to add highlight effect
                        if (currentPrices[symbol] && currentPrices[symbol].Price !== ticker.Price) {
                            const card = document.getElementById('card-' + symbol);
                            card.classList.add('updated');
                            setTimeout(() => card.classList.remove('updated'), 500);
                        }
                        
                        priceEl.textContent = ticker.price;
                        timeEl.textContent = new Date(ticker.lastUpdatedTimeUtc).toLocaleString();
                    }
                }
            });
            
            // Store current prices for comparison
            currentPrices = JSON.parse(JSON.stringify(prices));
            
            // Update stats
            updateStats(prices);
        }

        // Handle page visibility changes
        document.addEventListener('visibilitychange', function() {
            if (document.hidden && ws && ws.readyState === WebSocket.OPEN) {
                ws.close(1000, 'Page hidden');
            }
        });

        // Handle page unload
        window.addEventListener('beforeunload', function() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.close(1000, 'Page closing');
            }
        });

        // Connect when page loads
        connect();
    </script>
</body>
</html>`

	w.Write([]byte(html))
}
