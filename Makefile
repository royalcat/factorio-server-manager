docker-build:
	@echo "Building Docker Image"
	@docker build -t factorio-server-manager:develop .

clean:
	@echo "Cleaning"
	@-rm -r build/
	@-rm app/bundle.js
	@-rm app/bundle.js.map
	@-rm app/style.css
	@-rm app/style.css.map
	@-rm -r app/assets/
	@-rm -r app/fonts/vendor/
	@-rm -r app/images/vendor/
	@-rm -rf node_modules/
	@-rm -r pkg/
	@-rm -r factorio-server-manager
