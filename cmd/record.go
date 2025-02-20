package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
)

func validatedFullName(name, namespace string, recordType string, truncate bool) string {
	// Let's determine the type of record for the log.
	recordKind := recordType
	if recordType == "" {
		recordKind = "PTR"
	}

	// Let's verify the length of each component of the name
	components := []string{name}
	if namespace != "" {
		components = append(components, namespace)
	}

	// Verify each component
	for _, component := range components {
		if len(component) > 63 {
			if truncate {
				// If a component is too long, truncate it
				hasher := sha256.New()
				hasher.Write([]byte(component))
				hash := hex.EncodeToString(hasher.Sum(nil))
				shortComponent := component[:51]
				truncatedComponent := fmt.Sprintf("%s-%s", shortComponent, hash[:8])
				log.Printf("✂️  DNS label '%s' exceeds length limit (63 chars) for %s record\n"+
					"                      └─ Truncated to: '%s'\n",
					component, recordKind, truncatedComponent)

				// Replace the original component with the truncated one
				if component == name {
					name = truncatedComponent
				} else {
					namespace = truncatedComponent
				}
			} else {
				log.Printf("DNS label '%s' exceeds length limit (63 chars) for %s record\n"+
					"                      └─ Record will not be published\n",
					component, recordKind)
				return ""
			}
		}
	}

	// Return the full name
	if namespace != "" {
		return fmt.Sprintf("%s.%s", name, namespace)
	}
	return name
}

func validatedRecord(name, namespace string, ttl int, recordType string, ip net.IP, truncate bool) string {
	fullname := validatedFullName(name, namespace, recordType, truncate)
	if fullname == "" {
		return ""
	}
	return fmt.Sprintf("%s.local. %d IN %s %s", fullname, ttl, recordType, ip)
}

func validatedPTRRecord(reverseIP string, ttl int, name, namespace string, truncate bool) string {
	// Validate the fullname
	fullname := validatedFullName(name, namespace, "", truncate)
	if fullname == "" {
		return ""
	}
	return fmt.Sprintf("%s %d IN PTR %s.local.", reverseIP, ttl, fullname)
}
