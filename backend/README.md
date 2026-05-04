# Leave Management Backend

Small Go backend for the leave-management gateway API.

Run it:

```bash
go run .
```

The server listens on `http://localhost:8080`.

Example:

```bash
curl http://localhost:8080/leave-types
curl http://localhost:8080/employees/emp-100/leave-requests
```
