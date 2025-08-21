# Trade Stream

A Go application for monitoring cryptocurrency prices using both Binance.US WebSocket streams and REST API endpoints, with an optional HTTP server for API access.

## Building, Running & Testing the Service

Download dependencies:

```bash
    go mod download
```

Configure the service:
 - check config/config.yaml
 - add or remove trading symbols you want to get the live price for

```bash
## we have four symbols by default
binance:
  ...
  symbols:
    - 'BTCUSDT'
    - 'ETHUSDT'
    - 'BTCUSD'
    - 'ETHUSD'
```

```bash
## removing all symbols will subscribe the service to all available symbols from binance.us
binance:
  ...
  symbols:
```

Build the service:
```bash
make build
```

Run the service
```bash
make run
```

Stop the service
```bash
make run
```

Run unit tests
```bash
make run-unit-tests
```

Run api tests
```bash
make run-api-tests
```

## Navigating the running application
baseUrl is localhost:8080
### Endpoints:
 - /health
   - health check endpoint used to monitor the health of the application
   - ex. curl `curl -X GET http://localhost:8080/health`
 - /prices
   - get live prices of symbols set in config.binance.symbols
   - ex. curl `curl -X GET http://localhost:8080/prices`
 - /price?symbols=[comma,separated,list]
   - get live prices of any subset of the symbols set in config.binance.symbols
   - ex. curl `curl -X GET http://localhost:8080/price?symbols=BTCUSD,BTCUSDT`

### Webpage
 - /live-prices
   - displays live prices of symbols set in config.binance.symbols

### Websocket
 - /ws/prices
   - websocket connection to supply live price updates for symbols set in config.binance.symbols
   - used by /lives-prices view

## Discussion of Implementation

The service initial loads config values of Symbols. The prices for those symbols are then 
loaded into an in memory data store the PriceManager which uses channels to update data in a thread-safe way,
using RWMutex write locks for any priceUpdates and read locks from any subscriber channels or from any direct
call to get the currentPrices via getCurrentPrices. 

The service also spins up go routines in the background to create a persistent connection with binance.us websocket
trade stream. Any priceUpdates received are sent to the PriceManager via channels. 

### PriceManager
 - priceManager has a singleUpdate thread and processes updates as it receives them
   - always assumes the latest trade event is the most valid (we are not checking trade timestamps, for example)
 - current no limit on the number of subscribers
   - could cause memory issues and the service to fail past a certain point, could limit subscriber limit in code
 - snapshots send data in memory
   - makes the logic for subscribers to consume priceUpdate very easy
   - comes at the cost of added memory overhead in the application
 - updates are blocked until a subscriber receives and processes the update
   - updates are lost if a subscriber channel is blocked, could be helped with added retry logic

### PriceLoader
- assumes that data from binance rest api will not clash with websocket stream data
  - initially populates data from rest api and assumes events from websocket stream will be after the data represented in the api
- price loader sets up long running go-routines that exit only on error or signal interrupt
  - there is no error handling / connection management around restarting this process if the websocket client connection throws an error
  - ideally this should be addressed in the websocket connection

### Websocket Connection -> Binance
 - Creates a new stream per limit set in config
   - binance has a limit of 1024 streams per a given websocket connection - have to create multiple connections if you go over this limit
 - no handling of websocket connection failure (...yet)

### Overall
 - Not necessarily robust -> areas of improvement
   - add retry logic for publishing to subscribers
   - add retry when loading data from binance rest api
   - add reconnection logic dealing with a websocket connection error
