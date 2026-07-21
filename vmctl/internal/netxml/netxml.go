// Package netxml does light, targeted parsing of libvirt network XML
// (`virsh net-dumpxml`) for the handful of fields both `setup` and `cleanup`
// need: static DHCP host reservations and the DHCP range. It intentionally
// avoids a full XML unmarshal (see design.md's Level 2 discussion) — this
// phase keeps the same regex-shaped approach the bash scripts used, now
// shared instead of duplicated.
package netxml

import "strings"

// ExtractHostEntries returns every `<host .../>` element in a libvirt
// network's XML, mirroring `grep -oP "<host [^>]*/>"`.
func ExtractHostEntries(xml string) []string {
	var entries []string
	for {
		start := strings.Index(xml, "<host ")
		if start == -1 {
			break
		}
		end := strings.Index(xml[start:], "/>")
		if end == -1 {
			break
		}
		entries = append(entries, xml[start:start+end+2])
		xml = xml[start+end+2:]
	}
	return entries
}

// Attr extracts the value of attr='...' from a `<host .../>` entry.
func Attr(entry, attr string) string {
	needle := attr + "='"
	start := strings.Index(entry, needle)
	if start == -1 {
		return ""
	}
	start += len(needle)
	end := strings.Index(entry[start:], "'")
	if end == -1 {
		return ""
	}
	return entry[start : start+end]
}

// FindHostEntryByName finds the `<host .../>` entry whose name attribute
// matches name, or "" if none does.
func FindHostEntryByName(xml, name string) string {
	for _, entry := range ExtractHostEntries(xml) {
		if Attr(entry, "name") == name {
			return entry
		}
	}
	return ""
}

// FindReservationOwnerByIP returns the name reserved for the given ip, or ""
// if the ip has no static reservation.
func FindReservationOwnerByIP(xml, ip string) string {
	for _, entry := range ExtractHostEntries(xml) {
		if Attr(entry, "ip") == ip {
			return Attr(entry, "name")
		}
	}
	return ""
}

// HasMAC reports whether any `<host .../>` entry already uses mac
// (case-insensitive), mirroring `grep -qi "mac='${mac}'"`.
func HasMAC(xml, mac string) bool {
	lowerXML := strings.ToLower(xml)
	needle := strings.ToLower("mac='" + mac + "'")
	return strings.Contains(lowerXML, needle)
}

// DHCPRange extracts the start/end of the first `<range .../>` element,
// mirroring `grep -oP '<range[^/]*/>' | head -n1`.
func DHCPRange(xml string) (start, end string) {
	idx := strings.Index(xml, "<range")
	if idx == -1 {
		return "", ""
	}
	closeIdx := strings.Index(xml[idx:], "/>")
	if closeIdx == -1 {
		return "", ""
	}
	entry := xml[idx : idx+closeIdx+2]
	return Attr(entry, "start"), Attr(entry, "end")
}
