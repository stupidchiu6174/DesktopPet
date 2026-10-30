// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "DesktopPet",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "DesktopPet", targets: ["DesktopPet"]),
        .executable(name: "DesktopPetApp", targets: ["DesktopPetApp"]),
    ],
    targets: [
        .target(name: "DesktopPet", resources: [.copy("Resources/Pet")], swiftSettings: [.swiftLanguageMode(.v5)]),
        .executableTarget(name: "DesktopPetApp", dependencies: ["DesktopPet"], swiftSettings: [.swiftLanguageMode(.v5)]),
        .testTarget(name: "DesktopPetTests", dependencies: ["DesktopPet"], swiftSettings: [.swiftLanguageMode(.v5)]),
    ]
)
