package coordinate

// Config is used to set the parameters of the Vivaldi-based coordinate mapping
// algorithm. The following references are called out at various points in the
// documentation here:
//
// [1] Dabek, Frank, et al. "Vivaldi: A decentralized network coordinate system."
//     ACM SIGCOMM Computer Communication Review. Vol. 34. No. 4. ACM, 2004.
// [2] Ledlie, Jonathan, Paul Gardner, and Margo I. Seltzer. "Network Coordinates
//     in the Wild." NSDI. Vol. 7. 2007.
// [3] Lee, Sanghwan, et al. "On suitability of Euclidean embedding for
//     host-based network coordinate systems." Networking, IEEE/ACM Transactions
//     on 18.1 (2010): 27-40.
type Config struct {
	// The dimensionality of the coordinate system. As discussed in [2], more
	// dimensions improves the accuracy of the estimates up to a point. In
	// particular, there was no noticeable improvement beyond 7 dimensions.
	Dimensionality uint

	// MaxVivaldiError is the default error value when a node hasn't yet made
	// any observations. It also serves as an upper limit on the error value in
	// case observations cause the error value to increase without bound.
	MaxVivaldiError float64

	// VivaldiCE is a tuning factor that controls the maximum impact an
	// observation can have on a node's confidence. See [1] for more details.
	VivaldiCE float64

	// VivaldiCC is a tuning factor that controls the maximum impact an
	// observation can have on a node's coordinate. See [1] for more details.
	VivaldiCC float64
}

// DefaultConfig returns a Config that has some default values suitable for
// basic testing of the algorithm, but not tuned to any particular type of cluster.
func DefaultConfig() *Config {
	return &Config{
		Dimensionality:  8,
		MaxVivaldiError: 1.5,
		VivaldiCE:       0.25,
		VivaldiCC:       0.25,
	}
}
