build: clean-images
	docker build --no-cache -t trade-stream:latest .

clean-images:
	@echo "Cleaning Docker images..."
	docker image rm $$(docker images --filter "reference=trade-stream" -aq)
	@echo "Docker images cleaned"

run:
	docker-compose up -d trade-stream

stop:
	docker-compose stop trade-stream

run-unit-tests:
	docker-compose run --rm unit-test

run-api-tests:
	docker-compose run --rm api-test
