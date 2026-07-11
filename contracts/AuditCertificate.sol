// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract AuditCertificate is ERC721, Ownable {
    uint256 private _nextTokenId;
    mapping(uint256 => string) private _tokenURIs;

    event CertificateMinted(address indexed recipient, uint256 indexed tokenId, string tokenURI);

    constructor() ERC721("Sertifikat Audit Digital", "ELYCERT") {}

    function mintCertificate(address recipient, string memory uri) public onlyOwner returns (uint256) {
        uint256 tokenId = _nextTokenId;
        _nextTokenId++;

        _safeMint(recipient, tokenId);
        _tokenURIs[tokenId] = uri;

        emit CertificateMinted(recipient, tokenId, uri);

        return tokenId;
    }

    function tokenURI(uint256 tokenId) public view override returns (string memory) {
        require(_exists(tokenId), "ERC721Metadata: URI query for nonexistent token");
        return _tokenURIs[tokenId];
    }
}
