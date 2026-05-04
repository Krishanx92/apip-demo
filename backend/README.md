# Leave Management Backend

Small Go backend for the leave-management gateway API.

Run it:

```bash
go run .
```

The server listens on `http://localhost:8088` by default. Override with `PORT` if needed.

Example:

```bash
curl http://localhost:8088/leave-types
curl http://localhost:8088/employees/emp-100/leave-requests
```
