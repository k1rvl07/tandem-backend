window.onload = function () {
	const ui = SwaggerUIBundle({
		url: "./doc.json",
		dom_id: "#swagger-ui",
		deepLinking: true,
		presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
		layout: "StandaloneLayout",
	});
	window.ui = ui;
};