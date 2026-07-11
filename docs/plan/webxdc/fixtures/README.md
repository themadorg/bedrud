# WebXDC fixtures

Source trees for packaging into `.xdc` during host development.

| Dir | Role |
|-----|------|
| `demo-echo/` | Minimal get_started-style collaborative echo |
| `hostile-probe/` | Manual security probes (network, parent, WebRTC) |

Package:

```bash
(cd demo-echo && zip -9 -r ../demo-echo.xdc .)
(cd hostile-probe && zip -9 -r ../hostile-probe.xdc .)
```

Do not commit binary `.xdc` unless CI e2e needs them. Host must still inject `webxdc.js`.
