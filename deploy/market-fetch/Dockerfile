FROM alpine:3.20
RUN apk add --no-cache python3 curl
COPY fetch.py /app/fetch.py
EXPOSE 8890
CMD ["python3", "-u", "/app/fetch.py"]
