package redis_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gopkg.in/mohandutt134/redis.v4"
	"gopkg.in/mohandutt134/redis.v4/internal/pool"
)

var _ = Describe("pool", func() {
	var client *redis.Client

	BeforeEach(func() {
		client = redis.NewClient(redisOptions())
		Expect(client.FlushDb().Err()).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.Close()).NotTo(HaveOccurred())
	})

	It("should respect max size", func() {
		perform(1000, func(id int) {
			val, err := client.Ping().Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("PONG"))
		})

		pool := client.Pool()
		Expect(pool.Len()).To(BeNumerically("<=", 10))
		Expect(pool.FreeLen()).To(BeNumerically("<=", 10))
		Expect(pool.Len()).To(Equal(pool.FreeLen()))
	})

	It("should respect max on multi", func() {
		perform(1000, func(id int) {
			var ping *redis.StatusCmd

			err := client.Watch(func(tx *redis.Tx) error {
				cmds, err := tx.MultiExec(func() error {
					ping = tx.Ping()
					return nil
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(cmds).To(HaveLen(1))
				return err
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(ping.Err()).NotTo(HaveOccurred())
			Expect(ping.Val()).To(Equal("PONG"))
		})

		pool := client.Pool()
		Expect(pool.Len()).To(BeNumerically("<=", 10))
		Expect(pool.FreeLen()).To(BeNumerically("<=", 10))
		Expect(pool.Len()).To(Equal(pool.FreeLen()))
	})

	It("should respect max on pipelines", func() {
		perform(1000, func(id int) {
			pipe := client.Pipeline()
			ping := pipe.Ping()
			cmds, err := pipe.Exec()
			Expect(err).NotTo(HaveOccurred())
			Expect(cmds).To(HaveLen(1))
			Expect(ping.Err()).NotTo(HaveOccurred())
			Expect(ping.Val()).To(Equal("PONG"))
			Expect(pipe.Close()).NotTo(HaveOccurred())
		})

		pool := client.Pool()
		Expect(pool.Len()).To(BeNumerically("<=", 10))
		Expect(pool.FreeLen()).To(BeNumerically("<=", 10))
		Expect(pool.Len()).To(Equal(pool.FreeLen()))
	})

	It("should respect max on pubsub", func() {
		connPool := client.Pool()
		connPool.(*pool.ConnPool).DialLimiter = nil

		perform(1000, func(id int) {
			pubsub := client.PubSub()
			Expect(pubsub.Subscribe()).NotTo(HaveOccurred())
			Expect(pubsub.Close()).NotTo(HaveOccurred())
		})

		Expect(connPool.Len()).To(Equal(connPool.FreeLen()))
		Expect(connPool.Len()).To(BeNumerically("<=", 10))
	})

	It("should remove broken connections", func() {
		cn, err := client.Pool().Get()
		Expect(err).NotTo(HaveOccurred())
		cn.NetConn = &badConn{}
		Expect(client.Pool().Put(cn)).NotTo(HaveOccurred())

		err = client.Ping().Err()
		Expect(err).To(MatchError("bad connection"))

		val, err := client.Ping().Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal("PONG"))

		pool := client.Pool()
		Expect(pool.Len()).To(Equal(1))
		Expect(pool.FreeLen()).To(Equal(1))

		stats := pool.Stats()
		Expect(stats.Requests).To(Equal(uint32(4)))
		Expect(stats.Hits).To(Equal(uint32(2)))
		Expect(stats.Timeouts).To(Equal(uint32(0)))
	})

	It("should reuse connections", func() {
		for i := 0; i < 100; i++ {
			val, err := client.Ping().Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("PONG"))
		}

		pool := client.Pool()
		Expect(pool.Len()).To(Equal(1))
		Expect(pool.FreeLen()).To(Equal(1))

		stats := pool.Stats()
		Expect(stats.Requests).To(Equal(uint32(101)))
		Expect(stats.Hits).To(Equal(uint32(100)))
		Expect(stats.Timeouts).To(Equal(uint32(0)))
	})
})
