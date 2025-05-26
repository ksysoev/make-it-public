package edge

const htmlErrorTemplate502 = `<!DOCTYPE html>
<html>
<head>
	<title>502 Bad Gateway</title>
</head>
<body>
	<h1>502 Bad Gateway</h1>
	<p>The server received an invalid response from the upstream server.</p>
	<p>Please try again later.</p>
</body>
</html>`

const htmlErrorTemplate404 = `<!DOCTYPE html>
<html>
<head>
	<title>404 Not Found</title>
</head>
<body>
	<h1>404 Not Found</h1>
	<p>The requested resource could not be found on this server.</p>
	<p>Please check the URL and try again.</p>
</body>
</html>`
