package workerpool

type Config struct {
    WorkerLimit int `json:"workerLimit"`
    WorkerTimeoutSeconds int `json:"workerTimeoutSeconds"`
}

func NewDefaultConfig() *Config {
    return &Config{
        WorkerLimit: 25,
        WorkerTimeoutSeconds: 25,
    }
}