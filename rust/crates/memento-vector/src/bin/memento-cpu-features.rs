fn main() {
    #[cfg(target_arch = "x86_64")]
    println!(
        "arch=x86_64 sse4.2={} avx={} avx2={} fma={}",
        std::is_x86_feature_detected!("sse4.2"),
        std::is_x86_feature_detected!("avx"),
        std::is_x86_feature_detected!("avx2"),
        std::is_x86_feature_detected!("fma"),
    );

    #[cfg(target_arch = "aarch64")]
    println!(
        "arch=aarch64 neon={}",
        std::arch::is_aarch64_feature_detected!("neon"),
    );

    #[cfg(not(any(target_arch = "x86_64", target_arch = "aarch64")))]
    println!("arch=other");
}
