FROM docker.io/library/python:3.13-alpine

COPY . /app
WORKDIR /app
RUN pip install . && rm -rf /app

WORKDIR /data
ENTRYPOINT ["python", "-m", "tellama"]
