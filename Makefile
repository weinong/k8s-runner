all:
	go build github.com/weinong/k8s-runner
	docker build -t weinong/k8s-runner .

clean:
	rm k8s-runner
