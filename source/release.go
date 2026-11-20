// Package source fetches the Lean release of the claimed proof of the Riemann Hypothesis, checks its hashes, unpacks it and compares it with a file manifest.
//
// The release is 14 parts of one gzip tar archive at https://alpo.ge. The package knows the size and the SHA-256 of each part and of the archive, so a download that differs by one byte fails.
package source

// BaseURL is the address of the release.
const BaseURL = "https://alpo.ge"

// File is one file of the release, with its size in bytes and its SHA-256.
type File struct {
	Name   string
	Size   int64
	SHA256 string
}

// Release is a pinned release of the Lean source.
type Release struct {
	Name    string // the name of the release, as in the tags of riemann-lean
	Parts   []File // the parts, in order; their concatenation is Archive
	Archive File   // the gzip tar archive
	Note    File   // the note that comes with the release
}

const partSize = 18_000_000

// Release20261120 is the release of 2026-11-20.
var Release20261120 = Release{
	Name: "release-2026-11-20",
	Parts: []File{
		{"lean.tar.gz.part01", partSize, "86974b1c242ca7b6748f3299df4fc64ee0f6e06c776ac964fb64888101a3de77"},
		{"lean.tar.gz.part02", partSize, "1c2a2477354a9dfa97ff1ce54861e96ed885bd456fffbaaabc2028314a229c42"},
		{"lean.tar.gz.part03", partSize, "1465a23b1b579a881e24e5ffaf7c4c3727af28ea24741f1ec80e1d67947530a3"},
		{"lean.tar.gz.part04", partSize, "3929e327eecb4e2c5c7cae30e104e6a4a52fa6b82d28325bcda8e9da1c86893b"},
		{"lean.tar.gz.part05", partSize, "f25d7e80f7c4ad6d674aa18b323ca760f848857c8755501c149317c38a66c75d"},
		{"lean.tar.gz.part06", partSize, "a8c651a68327811b37dce3cca8eda5e0b2102fa227c0d395cbb7553e4dd69939"},
		{"lean.tar.gz.part07", partSize, "65c7e502a9b236a0f68d9e0aa6e089d4c09b9fc7a0960a149384063ccbe5e869"},
		{"lean.tar.gz.part08", partSize, "71b027ad6e4179c3a50ce72921e3fec0a2dd751ec7472ac2c644ad145a50ec7e"},
		{"lean.tar.gz.part09", partSize, "47718977ebaa8613f1647e93134479780db9320c5fa911463fd312a6257b796f"},
		{"lean.tar.gz.part10", partSize, "1d85ece78e0779cec26b0cce29c6462d9f90180e56d78120b152df101715240b"},
		{"lean.tar.gz.part11", partSize, "cf83a159d1a9540012229c376562587dd966da10cd657cbcef53acfd03232448"},
		{"lean.tar.gz.part12", partSize, "1aa5415bd2db83b2689afeeb946419303b3c4f599ae790ad74d38465f768b502"},
		{"lean.tar.gz.part13", partSize, "2de4edbce8ebdbfd0b94bc89bab518c7294209f408ec5b14a7be9e4832d9da81"},
		{"lean.tar.gz.part14", 5_275_554, "53f8ea43a9b5dcab97b84a52f3ac2848711c844a5420a4c56418093bacb3c6f6"},
	},
	Archive: File{"lean.tar.gz", 239_275_554, "9631b4f020199e580ba86178cb56c6e9b09f690c2369a80dc4334aa93f49c7fc"},
	Note:    File{"remark.pdf", 245_114, "3fc97784b71ddd44ff8aa0a177e99622a1c61b13cfa843a0989ee4a7a0cfeb8d"},
}
