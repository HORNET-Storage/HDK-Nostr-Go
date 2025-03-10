# Test Client

This is just a very crude test client used for testing parts of the SDK along side hornet storage
Please do not re-use the keys, they are dummy keys that were generated entirely for this test environment
Make sure you read what the commands do before executing them as some create files for debug purposes

### Dummy Keypair
npub128qpftun6mzv3gh7pfcsq3sefgqwruz43kuhhu78hq99jeak5a0sstnu23
nsec1yas03jagdjsr8su00g92jurf7am3dldvu9tckyz796z8efpa594qp2nelz

## Commands
These are useful commands for debugging the hornet storage sdk along side a hornet storage relay. They are hard coded to work with a locally hosted hornet storage relay using the dummy keypair stated above.

### Download
download <scionic_root>

This will download a scionic merkle tree from a locally hosted hornet storage relay using the dummy keypair with the specified <scionic_root>

### Upload
upload <path>

This will create a scionic merkle tree from the directory of file specified by <path> and upload it to a locally hosted hornet storage relay using the dummy keypair