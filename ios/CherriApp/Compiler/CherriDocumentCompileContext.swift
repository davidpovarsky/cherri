import Foundation

/// The actual editor document and both revisions travel through the shared
/// compiler. An unsaved document has an empty URI and no filesystem authority.
struct CherriDocumentCompileContext: Codable, Equatable, Sendable {
    let uri: String
    let documentID: UUID
    let revision: UInt64
    let assetRevision: UInt64

    enum CodingKeys: String, CodingKey {
        case uri, revision, assetRevision
        case documentID = "documentId"
    }

    static func == (lhs: Self, rhs: Self) -> Bool {
        lhs.uri.utf16.elementsEqual(rhs.uri.utf16)
            && lhs.documentID == rhs.documentID
            && lhs.revision == rhs.revision
            && lhs.assetRevision == rhs.assetRevision
    }

    func advancingSourceRevision() -> Self {
        Self(uri: uri, documentID: documentID, revision: revision &+ 1,
             assetRevision: assetRevision)
    }

    var fileURL: URL? {
        guard !uri.isEmpty, let url = URL(string: uri), url.isFileURL else { return nil }
        return url
    }
}

/// These are process-wide counters from the existing shared Go asset cache.
/// They are observations, never an invented per-request hit/miss result.
struct CherriAssetCacheMetrics: Decodable, Equatable, Sendable {
    let metadataChecks: UInt64
    let hits: UInt64
    let misses: UInt64
    let reads: UInt64
    let readBytes: UInt64
    let encodes: UInt64
    let invalidations: UInt64
    let evictions: UInt64
    let residentBytes: Int64
    let entries: Int

    enum CodingKeys: String, CodingKey {
        case metadataChecks = "metadata_checks"
        case readBytes = "read_bytes"
        case residentBytes = "resident_bytes"
        case hits, misses, reads, encodes, invalidations, evictions, entries
    }
}
