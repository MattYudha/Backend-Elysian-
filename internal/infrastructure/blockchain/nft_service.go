package blockchain

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math/big"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// AuditCertificateABI is the contract ABI for AuditCertificate ERC-721
const AuditCertificateABI = `[
	{"inputs":[],"stateMutability":"nonpayable","type":"constructor"},
	{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"approved","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"Approval","type":"event"},
	{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"operator","type":"address"},{"indexed":false,"internalType":"bool","name":"approved","type":"bool"}],"name":"ApprovalForAll","type":"event"},
	{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"recipient","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"},{"indexed":false,"internalType":"string","name":"tokenURI","type":"string"}],"name":"CertificateMinted","type":"event"},
	{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"previousOwner","type":"address"},{"indexed":true,"internalType":"address","name":"newOwner","type":"address"}],"name":"OwnershipTransferred","type":"event"},
	{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"Transfer","type":"event"},
	{"inputs":[{"internalType":"address","name":"recipient","type":"address"},{"internalType":"string","name":"uri","type":"string"}],"name":"mintCertificate","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
	{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"tokenURI","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"}
]`

// NFTMetadata matches standard ERC-721 Metadata format
type NFTMetadata struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Image       string                 `json:"image"`
	Attributes  []map[string]interface{} `json:"attributes"`
}

type NFTService struct {
	client          *ethclient.Client
	contractAddress common.Address
	privateKey      *ecdsa.PrivateKey
	abi             abi.ABI
	pinataAPIKey    string
	pinataSecretKey string
	network         string
}

func NewNFTService(rpcURL, contractAddr, privateKeyHex, pinataKey, pinataSecret, network string) (*NFTService, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to blockchain client: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(AuditCertificateABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	var pk *ecdsa.PrivateKey
	if privateKeyHex != "" {
		pk, err = crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	return &NFTService{
		client:          client,
		contractAddress: common.HexToAddress(contractAddr),
		privateKey:      pk,
		abi:             parsedABI,
		pinataAPIKey:    pinataKey,
		pinataSecretKey: pinataSecret,
		network:         network,
	}, nil
}

// GenerateQRCode creates a PNG QR code for a given document URL
func (s *NFTService) GenerateQRCode(url string) ([]byte, error) {
	pngBytes, err := qrcode.Encode(url, qrcode.Medium, 150)
	if err != nil {
		return nil, fmt.Errorf("failed to generate qr code: %w", err)
	}
	return pngBytes, nil
}

// GenerateCertificateCover constructs a premium, high-tech vertical certificate cover (PNG)
func (s *NFTService) GenerateCertificateCover(taskID string, documentTitle string, totalIssues int, qrPNG []byte) ([]byte, error) {
	// 1. Create canvas 600 x 850
	width, height := 600, 850
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Colors
	bgDark := color.RGBA{11, 16, 31, 255}     // Dark Slate
	bgLight := color.RGBA{22, 33, 62, 255}     // Deep Navy
	cyanNeon := color.RGBA{0, 245, 255, 255}   // Neon Cyan
	cyanGlow := color.RGBA{0, 180, 200, 255}   // Soft Cyan
	goldColor := color.RGBA{233, 196, 106, 255} // Gold Accent
	whiteText := color.RGBA{255, 255, 255, 255}
	grayText := color.RGBA{150, 160, 175, 255}

	// 2. Draw smooth background gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Linear diagonal gradient
			ratio := float64(x+y) / float64(width+height)
			r := uint8(float64(bgDark.R)*(1-ratio) + float64(bgLight.R)*ratio)
			g := uint8(float64(bgDark.G)*(1-ratio) + float64(bgLight.G)*ratio)
			b := uint8(float64(bgDark.B)*(1-ratio) + float64(bgLight.B)*ratio)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	// 3. Draw dual elegant borders
	drawBorder(img, 15, 15, width-15, height-15, 2, cyanNeon)
	drawBorder(img, 20, 20, width-20, height-20, 1, cyanGlow)

	// Draw high-tech aesthetic corner decorations
	drawCornerBracket(img, 25, 25, 40, 40, 2, cyanNeon)
	drawCornerBracket(img, width-25, 25, width-40, 40, 2, cyanNeon)
	drawCornerBracket(img, 25, height-25, 40, height-40, 2, cyanNeon)
	drawCornerBracket(img, width-25, height-25, width-40, height-40, 2, cyanNeon)

	// 4. Draw Header / Logo
	drawBasicText(img, width/2-100, 60, "ELYSIAN VERIFIABLE TRUST", goldColor, 1)
	drawBasicText(img, width/2-150, 95, "SERTIFIKAT AUDIT DIGITAL", cyanNeon, 2)
	drawLine(img, width/2-180, 120, width/2+180, 120, goldColor)

	// 5. Draw Certificate Body Info
	drawBasicText(img, 50, 170, "ID TUGAS AUDIT:", grayText, 1)
	drawBasicText(img, 200, 170, taskID, whiteText, 1)

	drawBasicText(img, 50, 210, "DOKUMEN:", grayText, 1)
	titleDisplay := documentTitle
	if len(titleDisplay) > 35 {
		titleDisplay = titleDisplay[:32] + "..."
	}
	drawBasicText(img, 200, 210, titleDisplay, whiteText, 1)

	drawBasicText(img, 50, 250, "PENGUJI:", grayText, 1)
	drawBasicText(img, 200, 250, "SWARM AI COMPLIANCE ENGINE", goldColor, 1)

	drawBasicText(img, 50, 290, "TANGGAL:", grayText, 1)
	drawBasicText(img, 200, 290, time.Now().Format("2006-01-02 15:04:05 MST"), whiteText, 1)

	// 6. Draw glowing verification badge
	drawRect(img, width/2-180, 340, width/2+180, 430, color.RGBA{0, 245, 255, 30})
	drawBorder(img, width/2-180, 340, width/2+180, 430, 1, cyanNeon)

	statusStr := "DOKUMEN TERVERIFIKASI"
	issuesStr := fmt.Sprintf("TOTAL TEMUAN KEBIJAKAN: %d", totalIssues)
	if totalIssues == 0 {
		statusStr = "DOKUMEN 100% AMAN & LAYAK"
	}
	drawBasicText(img, width/2-110, 370, statusStr, cyanNeon, 1)
	drawBasicText(img, width/2-125, 400, issuesStr, goldColor, 1)

	// 7. Overlay generated QR Code
	if len(qrPNG) > 0 {
		qrImg, _, err := image.Decode(bytes.NewReader(qrPNG))
		if err == nil {
			// Center the QR Code
			qrX := width/2 - 75
			qrY := height - 320
			draw.Draw(img, image.Rect(qrX, qrY, qrX+150, qrY+150), qrImg, image.Point{}, draw.Over)
			
			// Border around QR Code
			drawBorder(img, qrX-2, qrY-2, qrX+152, qrY+152, 1, goldColor)
			drawBasicText(img, width/2-70, height-150, "PINDAI UNTUK VERIFIKASI", grayText, 1)
		}
	}

	// 8. Footer Blockchain details
	drawBasicText(img, width/2-160, height-80, "TEREGISTRASI PADA LEDGER CRYPTO ETHEREUM", goldColor, 1)
	drawBasicText(img, width/2-135, height-55, "CRYPTOGRAPHIC INTEGRITY GUARANTEED", grayText, 1)

	// 9. Encode to PNG bytes
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode png: %w", err)
	}

	return buf.Bytes(), nil
}

// UploadToIPFS uploads image cover bytes and JSON metadata to Pinata (IPFS)
func (s *NFTService) UploadToIPFS(ctx context.Context, imgBytes []byte, taskID, docTitle string, totalIssues int) (string, error) {
	// If Pinata credentials are empty or mock, return simulated IPFS CID
	if s.pinataAPIKey == "" || s.pinataAPIKey == "mock_pinata_api_key" {
		log.Printf("[IPFS-Mock] Pinata keys not configured. Simulating IPFS upload for task %s...", taskID)
		// Generate deterministic simulated hash based on taskID to look real
		simulatedCID := "Qm" + crypto.Keccak256Hash([]byte(taskID)).Hex()[2:44]
		log.Printf("[IPFS-Mock] Generated mock IPFS CID: ipfs://%s", simulatedCID)
		return simulatedCID, nil
	}

	// 1. Upload Cover Image to IPFS
	imageCID, err := s.uploadFileToPinata(ctx, imgBytes, fmt.Sprintf("audit_cover_%s.png", taskID), "image/png")
	if err != nil {
		return "", fmt.Errorf("failed to upload cover to Pinata: %w", err)
	}
	log.Printf("[IPFS] Uploaded cover successfully. CID: ipfs://%s", imageCID)

	// 2. Prepare metadata JSON
	metadata := NFTMetadata{
		Name:        fmt.Sprintf("Sertifikat Audit Digital - %s", docTitle),
		Description: fmt.Sprintf("Sertifikat Kepatuhan & Kelayakan Anggaran digital resmi yang dikeluarkan oleh Swarm AI Hub Elysian. Nomor Audit: %s. Kredensial ini ditandatangani secara kriptografis dan disimpan secara permanen pada desentralisasi IPFS.", taskID),
		Image:       fmt.Sprintf("ipfs://%s", imageCID),
		Attributes: []map[string]interface{}{
			{"trait_type": "Audit ID", "value": taskID},
			{"trait_type": "Judul Dokumen", "value": docTitle},
			{"trait_type": "Total Temuan", "value": totalIssues},
			{"trait_type": "Validator", "value": "Elysian Swarm AI Hub"},
			{"trait_type": "Status Keamanan", "value": map[bool]string{true: "LULUS_VERIFIKASI", false: "TEMUAN_DIRETROFIT"}[totalIssues == 0]},
			{"trait_type": "Waktu Audit", "value": time.Now().Format(time.RFC3339)},
		},
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata json: %w", err)
	}

	// 3. Upload Metadata JSON to IPFS
	metadataCID, err := s.uploadJSONToPinata(ctx, metadataBytes, fmt.Sprintf("metadata_%s.json", taskID))
	if err != nil {
		return "", fmt.Errorf("failed to upload metadata json: %w", err)
	}

	return metadataCID, nil
}

// MintNFT mints the certificate ERC-721 token on Ethereum Sepolia
func (s *NFTService) MintNFT(ctx context.Context, recipientAddress string, tokenURI string) (string, *big.Int, error) {
	if s.privateKey == nil {
		return "", nil, fmt.Errorf("private key not configured for minting")
	}

	publicKey := s.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", nil, fmt.Errorf("failed to cast public key")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	toAddr := common.HexToAddress(recipientAddress)

	// 1. Pack mintCertificate data
	data, err := s.abi.Pack("mintCertificate", toAddr, tokenURI)
	if err != nil {
		return "", nil, fmt.Errorf("failed to pack mintCertificate call: %w", err)
	}

	nonce, err := s.client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Dynamic EIP-1559 dynamic fee pricing
	tip, err := s.client.SuggestGasTipCap(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to suggest gas tip cap: %w", err)
	}

	// Add 50% buffer to tip
	tipBuffer := new(big.Int).Div(new(big.Int).Mul(tip, big.NewInt(150)), big.NewInt(100))
	minTip := big.NewInt(1500000000) // Minimum 1.5 Gwei tip cap
	if tipBuffer.Cmp(minTip) < 0 {
		tipBuffer = minTip
	}

	header, err := s.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get latest block: %w", err)
	}
	if header.BaseFee == nil {
		return "", nil, fmt.Errorf("base fee is nil in latest block header")
	}

	// MaxFeePerGas = (BaseFee * 2) + Tip
	maxFeePerGas := new(big.Int).Add(
		new(big.Int).Mul(header.BaseFee, big.NewInt(2)),
		tipBuffer,
	)

	gasLimit, err := s.client.EstimateGas(ctx, ethereum.CallMsg{
		From: fromAddress,
		To:   &s.contractAddress,
		Data: data,
	})
	if err != nil {
		log.Printf("[NFTService] Gas estimation failed for minting: %v. Using fallback of 300,000.", err)
		gasLimit = uint64(300000)
	} else {
		// Add 20% margin
		gasLimit = gasLimit * 12 / 10
	}

	chainID, err := s.client.NetworkID(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get network ID: %w", err)
	}

	// Create Dynamic Fee Transaction
	txData := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: tipBuffer,
		GasFeeCap: maxFeePerGas,
		Gas:       gasLimit,
		To:        &s.contractAddress,
		Value:     big.NewInt(0),
		Data:      data,
	}
	tx := types.NewTx(txData)

	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), s.privateKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to sign mint transaction: %w", err)
	}

	err = s.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send mint transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	log.Printf("[NFTService] Mint transaction submitted. Hash: %s. Waiting for verification...", txHash)

	// Wait for transaction receipt
	receipt, err := s.waitForReceipt(ctx, signedTx.Hash(), 120*time.Second)
	if err != nil {
		return txHash, nil, fmt.Errorf("timed out or failed waiting for mint confirmation: %w", err)
	}

	if receipt.Status != 1 {
		return txHash, nil, fmt.Errorf("transaction execution failed on-chain (reverted)")
	}

	// Parse logs to extract TokenID from Transfer event
	var mintedTokenID *big.Int = big.NewInt(0) // Default fallback
	for _, l := range receipt.Logs {
		if len(l.Topics) == 4 && l.Topics[0] == crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")) {
			// Transfer topic structure: Transfer(indexed from, indexed to, indexed tokenId)
			mintedTokenID = new(big.Int).SetBytes(l.Topics[3].Bytes())
			log.Printf("[NFTService] Extracted minted TokenID: %s", mintedTokenID.String())
			break
		}
	}

	return txHash, mintedTokenID, nil
}

func (s *NFTService) waitForReceipt(ctx context.Context, hash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		receipt, err := s.client.TransactionReceipt(ctx, hash)
		if err == nil {
			return receipt, nil
		}
		if err != ethereum.NotFound {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for receipt")
		case <-time.After(2 * time.Second):
			continue
		}
	}
}

// uploadFileToPinata posts a file to Pinata IPFS
func (s *NFTService) uploadFileToPinata(ctx context.Context, fileBytes []byte, filename, contentType string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, bytes.NewReader(fileBytes)); err != nil {
		return "", err
	}

	// Add pinning metadata
	metaPart, err := writer.CreateFormField("pinataMetadata")
	if err == nil {
		metaJSON := fmt.Sprintf(`{"name":"%s"}`, filename)
		metaPart.Write([]byte(metaJSON))
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.pinata.cloud/pinning/pinFileToIPFS", body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("pinata_api_key", s.pinataAPIKey)
	req.Header.Set("pinata_secret_api_key", s.pinataSecretKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pinata upload rejected (status %d): %s", resp.StatusCode, string(respBytes))
	}

	var response struct {
		IpfsHash string `json:"IpfsHash"`
	}
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return "", err
	}

	return response.IpfsHash, nil
}

// uploadJSONToPinata uploads JSON metadata bytes directly to Pinata IPFS
func (s *NFTService) uploadJSONToPinata(ctx context.Context, jsonBytes []byte, filename string) (string, error) {
	payload := map[string]interface{}{
		"pinataContent": json.RawMessage(jsonBytes),
		"pinataMetadata": map[string]interface{}{
			"name": filename,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.pinata.cloud/pinning/pinJSONToIPFS", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("pinata_api_key", s.pinataAPIKey)
	req.Header.Set("pinata_secret_api_key", s.pinataSecretKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pinata upload json rejected (status %d): %s", resp.StatusCode, string(respBytes))
	}

	var response struct {
		IpfsHash string `json:"IpfsHash"`
	}
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return "", err
	}

	return response.IpfsHash, nil
}

// --- Graphical Draw Utilities for basic geometric rendering ---

func drawBorder(img *image.RGBA, x1, y1, x2, y2, thickness int, col color.Color) {
	for t := 0; t < thickness; t++ {
		// Top & Bottom Horizontal lines
		for x := x1; x <= x2; x++ {
			img.Set(x, y1+t, col)
			img.Set(x, y2-t, col)
		}
		// Left & Right Vertical lines
		for y := y1; y <= y2; y++ {
			img.Set(x1+t, y, col)
			img.Set(x2-t, y, col)
		}
	}
}

func drawCornerBracket(img *image.RGBA, x, y, bx, by, thickness int, col color.Color) {
	// Draw horizontal line from corner
	xStep := 1
	if bx < x {
		xStep = -1
	}
	for xi := x; xi != bx+xStep; xi += xStep {
		for t := 0; t < thickness; t++ {
			img.Set(xi, y+t, col)
		}
	}

	// Draw vertical line from corner
	yStep := 1
	if by < y {
		yStep = -1
	}
	for yi := y; yi != by+yStep; yi += yStep {
		for t := 0; t < thickness; t++ {
			img.Set(x+t, yi, col)
		}
	}
}

func drawLine(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	if y1 == y2 { // Horizontal
		for x := x1; x <= x2; x++ {
			img.Set(x, y1, col)
		}
	} else if x1 == x2 { // Vertical
		for y := y1; y <= y2; y++ {
			img.Set(x1, y, col)
		}
	}
}

func drawRect(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			img.Set(x, y, col)
		}
	}
}

func drawBasicText(img *image.RGBA, x, y int, text string, col color.Color, scale int) {
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}

	if scale <= 1 {
		drawer.DrawString(text)
		return
	}

	// For higher scale, we draw character by character onto a temp image, then scale & draw it
	tempImg := image.NewRGBA(image.Rect(0, 0, len(text)*7, 13))
	tempDrawer := &font.Drawer{
		Dst:  tempImg,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(0, 11),
	}
	tempDrawer.DrawString(text)

	// Scale and overlay on destination
	for ty := 0; ty < 13; ty++ {
		for tx := 0; tx < len(text)*7; tx++ {
			c := tempImg.RGBAAt(tx, ty)
			if c.A > 0 {
				// Fill scaled blocks
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						img.Set(x+tx*scale+sx, y-10*scale+ty*scale+sy, c)
					}
				}
			}
		}
	}
}
