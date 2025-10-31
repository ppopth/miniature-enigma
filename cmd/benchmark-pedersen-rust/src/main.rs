use blst::min_pk::{PublicKey, SecretKey, Signature};
use clap::Parser;
use curve25519_dalek::constants::RISTRETTO_BASEPOINT_POINT;
use curve25519_dalek::ristretto::RistrettoPoint;
use curve25519_dalek::scalar::Scalar;
use curve25519_dalek::traits::MultiscalarMul;
use rand::RngCore;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::fs;
use std::time::Instant;

/// Benchmark tool for Pedersen commitment verification using Rust and curve25519-dalek
#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    /// Message chunk size in bytes (not network chunk size)
    #[arg(long, default_value_t = 1024)]
    chunk_size: usize,

    /// Number of chunks to benchmark
    #[arg(long, default_value_t = 8)]
    num_chunks: usize,

    /// Number of iterations per benchmark
    #[arg(long, default_value_t = 100)]
    iterations: usize,

    /// Output file for benchmark results
    #[arg(long, default_value = "pedersen_benchmark_rust.json")]
    output: String,
}

#[derive(Serialize, Deserialize, Debug)]
struct BenchmarkResult {
    chunk_size: usize,
    elements_per_chunk: usize,
    num_chunks: usize,
    iterations: usize,
    generate_extras_ns: u64,
    verify_ns: u64,
}

/// Pedersen commitment verifier using Ristretto255
struct PedersenVerifier {
    chunk_size: usize,
    elements_per_chunk: usize,
    generators: Vec<RistrettoPoint>,
    bls_secret_key: SecretKey,
}

/// Extra data structure containing commitments and BLS signature
#[derive(Clone)]
struct ExtraData {
    publisher_id: u32,
    commitments: Vec<Vec<u8>>,
    bls_signature: Vec<u8>,
}

impl PedersenVerifier {
    /// Create a new Pedersen verifier with network chunk size
    fn new(network_chunk_size: usize, elements_per_chunk: usize) -> Self {
        // Generate deterministic generator points
        let mut generators = Vec::with_capacity(elements_per_chunk);
        for i in 0..elements_per_chunk {
            generators.push(Self::generate_point(i));
        }

        // Generate BLS secret key
        let mut ikm = [0u8; 32];
        rand::thread_rng().fill_bytes(&mut ikm);
        let bls_secret_key = SecretKey::key_gen(&ikm, &[]).unwrap();

        Self {
            chunk_size: network_chunk_size,
            elements_per_chunk,
            generators,
            bls_secret_key,
        }
    }

    /// Generate a deterministic generator point using hash-to-curve
    fn generate_point(index: usize) -> RistrettoPoint {
        let seed = format!("pedersen_generator_{}", index);
        let mut hasher = Sha256::new();
        hasher.update(seed.as_bytes());
        let hash = hasher.finalize();

        // Use hash as scalar to multiply basepoint
        // This creates a deterministic but unpredictable point
        let scalar = Scalar::from_bytes_mod_order(hash.into());
        &scalar * &RISTRETTO_BASEPOINT_POINT
    }

    /// Split chunk data into field elements (scalars)
    fn split_into_elements(&self, data: &[u8]) -> Vec<Scalar> {
        let bytes_per_element = self.chunk_size / self.elements_per_chunk;
        let mut elements = Vec::with_capacity(self.elements_per_chunk);

        for i in 0..self.elements_per_chunk {
            let start = i * bytes_per_element;
            let end = start + bytes_per_element;

            if end <= data.len() {
                // Take up to 32 bytes for the scalar
                let mut bytes = [0u8; 32];
                let chunk_bytes = &data[start..end];
                let copy_len = chunk_bytes.len().min(32);
                bytes[..copy_len].copy_from_slice(&chunk_bytes[..copy_len]);

                // Create scalar from bytes (mod order)
                elements.push(Scalar::from_bytes_mod_order(bytes));
            } else {
                elements.push(Scalar::ZERO);
            }
        }

        elements
    }

    /// Create a Pedersen commitment: C = Σ(vi * Gi)
    /// Uses multiscalar multiplication for optimal performance
    fn commit(&self, values: &[Scalar]) -> RistrettoPoint {
        let num_values = values.len().min(self.generators.len());
        RistrettoPoint::multiscalar_mul(&values[..num_values], &self.generators[..num_values])
    }

    /// Serialize a commitment to bytes
    fn serialize_commitment(&self, point: &RistrettoPoint) -> Vec<u8> {
        point.compress().to_bytes().to_vec()
    }

    /// Parse a commitment from bytes
    fn parse_commitment(&self, data: &[u8]) -> Option<RistrettoPoint> {
        if data.len() != 32 {
            return None;
        }

        let mut bytes = [0u8; 32];
        bytes.copy_from_slice(data);

        curve25519_dalek::ristretto::CompressedRistretto(bytes)
            .decompress()
    }

    /// Generate BLS signature over commitments
    fn generate_bls_signature(&self, commitments: &[Vec<u8>]) -> Vec<u8> {
        // Hash all commitments together
        let mut hasher = Sha256::new();
        for commitment in commitments {
            hasher.update(commitment);
        }
        let message = hasher.finalize();

        // Sign the hash
        let signature = self.bls_secret_key.sign(&message, &[], &[]);
        signature.to_bytes().to_vec()
    }

    /// Verify BLS signature over commitments
    fn verify_bls_signature(
        &self,
        commitments: &[Vec<u8>],
        signature_bytes: &[u8],
        public_key: &PublicKey,
    ) -> bool {
        // Hash all commitments together
        let mut hasher = Sha256::new();
        for commitment in commitments {
            hasher.update(commitment);
        }
        let message = hasher.finalize();

        // Parse signature
        let signature = match Signature::from_bytes(signature_bytes) {
            Ok(sig) => sig,
            Err(_) => return false,
        };

        // Verify signature
        signature.verify(true, &message, &[], &[], public_key, true) == blst::BLST_ERROR::BLST_SUCCESS
    }

    /// Serialize extra data to bytes
    fn serialize_extra_data(&self, extra: &ExtraData) -> Vec<u8> {
        let num_commitments = extra.commitments.len() as u32;
        let element_size = 32; // Ristretto255 compressed point size
        let total_size = 8 + (num_commitments as usize * element_size) + 96;

        let mut result = vec![0u8; total_size];
        let mut offset = 0;

        // Write publisher ID (4 bytes)
        result[offset..offset + 4].copy_from_slice(&extra.publisher_id.to_be_bytes());
        offset += 4;

        // Write number of commitments (4 bytes)
        result[offset..offset + 4].copy_from_slice(&num_commitments.to_be_bytes());
        offset += 4;

        // Write commitments
        for commitment in &extra.commitments {
            result[offset..offset + element_size].copy_from_slice(commitment);
            offset += element_size;
        }

        // Write BLS signature (96 bytes)
        result[offset..offset + 96].copy_from_slice(&extra.bls_signature);

        result
    }

    /// Parse extra data from bytes
    fn parse_extra_data(&self, data: &[u8]) -> Option<ExtraData> {
        let element_size = 32;
        let min_size = 8 + element_size + 96;

        if data.len() < min_size {
            return None;
        }

        let mut offset = 0;

        // Read publisher ID (4 bytes)
        let publisher_id = u32::from_be_bytes([data[0], data[1], data[2], data[3]]);
        offset += 4;

        // Read number of commitments (4 bytes)
        let num_commitments = u32::from_be_bytes([data[4], data[5], data[6], data[7]]);
        offset += 4;

        let expected_size = 8 + (num_commitments as usize * element_size) + 96;
        if data.len() != expected_size {
            return None;
        }

        // Read commitments
        let mut commitments = Vec::with_capacity(num_commitments as usize);
        for _ in 0..num_commitments {
            let commitment = data[offset..offset + element_size].to_vec();
            commitments.push(commitment);
            offset += element_size;
        }

        // Read BLS signature (96 bytes)
        let bls_signature = data[offset..offset + 96].to_vec();

        Some(ExtraData {
            publisher_id,
            commitments,
            bls_signature,
        })
    }

    /// Generate extra fields for all chunks
    fn generate_extras(&self, chunks: &[Vec<u8>]) -> Vec<Vec<u8>> {
        if chunks.is_empty() {
            return vec![];
        }

        // Generate commitments for all chunks
        let commitments: Vec<Vec<u8>> = chunks
            .iter()
            .map(|chunk_data| {
                let elements = self.split_into_elements(chunk_data);
                let commitment = self.commit(&elements);
                self.serialize_commitment(&commitment)
            })
            .collect();

        // Generate BLS signature over all commitments
        let bls_signature = self.generate_bls_signature(&commitments);

        // Create extra data structure
        let extra_data = ExtraData {
            publisher_id: 0,
            commitments,
            bls_signature,
        };

        // Serialize and return same extra data for all chunks
        let serialized = self.serialize_extra_data(&extra_data);
        vec![serialized; chunks.len()]
    }

    /// Verify a chunk with its coefficients and extra data
    fn verify(
        &self,
        chunk_data: &[u8],
        coeffs: &[Scalar],
        extra: &[u8],
        public_key: &PublicKey,
    ) -> bool {
        // Parse extra data
        let extra_data = match self.parse_extra_data(extra) {
            Some(data) => data,
            None => return false,
        };

        // Check coefficient count
        if coeffs.len() != extra_data.commitments.len() {
            return false;
        }

        // Verify BLS signature
        if !self.verify_bls_signature(&extra_data.commitments, &extra_data.bls_signature, public_key)
        {
            return false;
        }

        // Compute commitment for this chunk
        let elements = self.split_into_elements(chunk_data);
        let computed_commitment = self.commit(&elements);

        // Parse all stored commitments
        let mut stored_commitments = Vec::with_capacity(extra_data.commitments.len());
        for commitment_bytes in &extra_data.commitments {
            match self.parse_commitment(commitment_bytes) {
                Some(c) => stored_commitments.push(c),
                None => return false,
            }
        }

        // Compute linear combination of stored commitments using multiscalar_mul
        let expected_commitment = RistrettoPoint::multiscalar_mul(coeffs, &stored_commitments);

        // Verify commitments match
        computed_commitment == expected_commitment
    }
}

fn main() {
    let args = Args::parse();

    let message_bytes_per_element = 32;
    let network_bytes_per_element = 33;

    // Validate chunk size
    if args.chunk_size % message_bytes_per_element != 0 {
        eprintln!(
            "Error: Message chunk size {} must be a multiple of {} bytes per element",
            args.chunk_size, message_bytes_per_element
        );
        std::process::exit(1);
    }

    let elements_per_chunk = args.chunk_size / message_bytes_per_element;
    let network_chunk_size = elements_per_chunk * network_bytes_per_element;

    println!("Benchmarking PedersenVerifier (Rust + curve25519-dalek) with:");
    println!("  Message chunk size: {} bytes", args.chunk_size);
    println!("  Network chunk size: {} bytes", network_chunk_size);
    println!("  Elements per chunk: {}", elements_per_chunk);
    println!("  Number of chunks: {}", args.num_chunks);
    println!("  Iterations: {}", args.iterations);
    println!();

    // Create verifier
    let verifier = PedersenVerifier::new(network_chunk_size, elements_per_chunk);

    // Get public key for verification
    let public_key = verifier.bls_secret_key.sk_to_pk();

    // Generate random test chunks: create message chunks and convert to network chunks
    // This matches what rlnc.go does in GenerateThenAddChunks
    let mut rng = rand::thread_rng();
    let chunks: Vec<Vec<u8>> = (0..args.num_chunks)
        .map(|_| {
            // Generate random message chunk
            let mut message_chunk = vec![0u8; args.chunk_size];
            rng.fill_bytes(&mut message_chunk);

            // Convert message chunk to network chunk
            // Message: 32 bytes per element, Network: 33 bytes per element
            let mut network_chunk = Vec::with_capacity(network_chunk_size);
            for i in 0..elements_per_chunk {
                let start = i * message_bytes_per_element;
                let end = start + message_bytes_per_element;
                // Copy 32 bytes of message data
                network_chunk.extend_from_slice(&message_chunk[start..end]);
                // Add 1 padding byte to make it 33 bytes per element
                network_chunk.push(0);
            }

            network_chunk
        })
        .collect();

    // Benchmark GenerateExtras
    print!("Benchmarking GenerateExtras... ");
    let start = Instant::now();
    let mut extras = vec![];
    for _ in 0..args.iterations {
        extras = verifier.generate_extras(&chunks);
    }
    let generate_extras_duration = start.elapsed() / args.iterations as u32;
    println!("{:?}", generate_extras_duration);

    // Benchmark Verify
    print!("Benchmarking Verify... ");

    // Generate coefficients for testing (2 bytes each, matching Go implementation)
    let coeffs: Vec<Scalar> = (0..args.num_chunks)
        .map(|_| {
            let mut bytes = [0u8; 32];
            // Use only 2 random bytes to match Go benchmark (creates small coefficients)
            rng.fill_bytes(&mut bytes[0..2]);
            Scalar::from_bytes_mod_order(bytes)
        })
        .collect();

    // Compute linear combination of chunks using the coefficients
    // This creates a coded chunk: chunk_data = Σ(coeff_i * chunk_i)
    let mut combined_elements = vec![Scalar::ZERO; elements_per_chunk];
    for (chunk_idx, chunk) in chunks.iter().enumerate() {
        let elements = verifier.split_into_elements(chunk);
        for (elem_idx, elem) in elements.iter().enumerate() {
            combined_elements[elem_idx] += coeffs[chunk_idx] * elem;
        }
    }

    // Convert combined elements back to bytes (network chunk format)
    let mut combined_chunk = Vec::with_capacity(network_chunk_size);
    for elem in &combined_elements {
        let elem_bytes = elem.to_bytes();
        // Take 32 bytes and add 1 padding byte for network format (33 bytes per element)
        combined_chunk.extend_from_slice(&elem_bytes);
        combined_chunk.push(0);
    }

    // First verify that the test chunk actually verifies correctly
    if !verifier.verify(&combined_chunk, &coeffs, &extras[0], &public_key) {
        eprintln!("Error: Test chunk verification failed");
        std::process::exit(1);
    }

    let start = Instant::now();
    for _ in 0..args.iterations {
        let _ = verifier.verify(&combined_chunk, &coeffs, &extras[0], &public_key);
    }
    let verify_duration = start.elapsed() / args.iterations as u32;
    println!("{:?}", verify_duration);

    // Create result
    let result = BenchmarkResult {
        chunk_size: network_chunk_size,
        elements_per_chunk,
        num_chunks: args.num_chunks,
        iterations: args.iterations,
        generate_extras_ns: generate_extras_duration.as_nanos() as u64,
        verify_ns: verify_duration.as_nanos() as u64,
    };

    // Write to file
    let json = serde_json::to_string_pretty(&result).unwrap();
    fs::write(&args.output, json).expect("Failed to write output file");

    println!("\nBenchmark results written to: {}", args.output);
}
