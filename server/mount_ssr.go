//go:build !wasm

package userserver

func (m *Module) wrapSSR(content string) string {
	return `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Login</title>
    <style>
        body { font-family: sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #f0f2f5; }
        .container { background: white; padding: 2rem; border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); width: 100%; max-width: 400px; }
        form { display: flex; flex-direction: column; gap: 1rem; }
        input { padding: 0.75rem; border: 1px solid #ddd; border-radius: 4px; }
        button { padding: 0.75rem; background: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer; }
        button:hover { background: #0056b3; }
        .oauth { margin-top: 1.5rem; border-top: 1px solid #eee; padding-top: 1rem; display: flex; flex-direction: column; gap: 0.5rem; }
        .oauth a { text-align: center; text-decoration: none; color: #555; border: 1px solid #ddd; padding: 0.5rem; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="container">
        ` + content + `
    </div>
</body>
</html>`
}
