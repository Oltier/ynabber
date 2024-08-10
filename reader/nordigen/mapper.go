package nordigen

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

type Mapper interface {
	Map(ynabber.Account, nordigen.Transaction) (ynabber.Transaction, error)
}

// Mapper returns a mapper to transform the banks transaction to Ynabber
func (r Reader) Mapper() Mapper {
	switch r.Config.Nordigen.BankID {
	case "NORDEA_NDEADKKK":
		return Nordea{}

	default:
		return Default{
			PayeeSource:   r.Config.Nordigen.PayeeSource,
			TransactionID: r.Config.Nordigen.TransactionID,
		}
	}
}

func parseAmount(t nordigen.Transaction) (float64, error) {
	amount, err := strconv.ParseFloat(t.TransactionAmount.Amount, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to convert string to float: %w", err)
	}
	return amount, nil
}

func parseDate(t nordigen.Transaction) (time.Time, error) {
	valueDate, valueDateErr := time.Parse("2006-01-02", t.ValueDate)
	bookingDate, bookingDateErr := time.Parse("2006-01-02", t.BookingDate)
	re := regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}`)
	remittanceDateString := re.FindString(t.RemittanceInformationUnstructured)
	remittanceDate, remittanceDateErr := time.Parse("2006.01.02", remittanceDateString)

	// Handle parsing errors
	if valueDateErr != nil && bookingDateErr != nil && remittanceDateErr != nil {
		return time.Time{}, fmt.Errorf("failed to parse any dates")
	}

	// Initialize earliestDate to the first non-zero date
	earliestDate := time.Time{}

	if remittanceDateErr == nil {
		earliestDate = remittanceDate
	}
	if valueDateErr == nil && (earliestDate.IsZero() || valueDate.Before(earliestDate)) {
		earliestDate = valueDate
	}
	if bookingDateErr == nil && (earliestDate.IsZero() || bookingDate.Before(earliestDate)) {
		earliestDate = bookingDate
	}

	return earliestDate, nil
}

// Default mapping for all banks unless a more specific mapping exists
type Default struct {
	PayeeSource   []string
	TransactionID string
}

// Map t using the default mapper
func (mapper Default) Map(a ynabber.Account, t nordigen.Transaction) (ynabber.Transaction, error) {
	amount, err := parseAmount(t)
	if err != nil {
		return ynabber.Transaction{}, err
	}
	date, err := parseDate(t)
	if err != nil {
		return ynabber.Transaction{}, err
	}

	// Get the Payee from the first data source that returns data in the order
	// defined by config
	payee := ""
	for _, source := range mapper.PayeeSource {
		if payee == "" {
			switch source {
			// Unstructured should properly have been called "remittance" but
			// its not. Some banks use this field as Payee.
			case "unstructured":
				payee = t.RemittanceInformationUnstructured
				// Unstructured data may need some formatting, some banks
				// inserts the amount and date which will cause every
				// transaction to create a new Payee
				payee = payeeStripNonAlphanumeric(payee)

			// Name is using either creditor or debtor as the payee
			case "name":
				if amount > 0 {
					if t.DebtorName != "" {
						payee = t.DebtorName
					} else if t.CreditorName != "" {
						payee = t.CreditorName
					}
				} else if t.CreditorName != "" {
					payee = t.CreditorName
				} else if t.DebtorName != "" {
					payee = t.DebtorName
				}

			// Additional uses AdditionalInformation as payee
			case "additional":
				payee = t.AdditionalInformation
			default:
				return ynabber.Transaction{}, fmt.Errorf("unrecognized PayeeSource: %s", source)
			}
		}
	}

	// Set the transaction ID according to config
	var id string
	switch mapper.TransactionID {
	case "InternalTransactionId":
		id = t.InternalTransactionId
	case "TransactionId":
		id = t.TransactionId
	default:
		return ynabber.Transaction{}, fmt.Errorf("unrecognized TransactionID: %s", mapper.TransactionID)
	}

	return ynabber.Transaction{
		Account: a,
		ID:      ynabber.ID(id),
		Date:    date,
		Payee:   ynabber.Payee(payee),
		Memo:    t.RemittanceInformationUnstructured,
		Amount:  ynabber.MilliunitsFromAmount(amount),
	}, nil
}

// Nordea implements a specific mapper for Nordea
type Nordea struct{}

// Map t using the Nordea mapper
func (mapper Nordea) Map(a ynabber.Account, t nordigen.Transaction) (ynabber.Transaction, error) {
	amount, err := parseAmount(t)
	if err != nil {
		return ynabber.Transaction{}, err
	}
	date, err := parseDate(t)
	if err != nil {
		return ynabber.Transaction{}, err
	}

	return ynabber.Transaction{
		Account: a,
		ID:      ynabber.ID(t.InternalTransactionId),
		Date:    date,
		Payee:   ynabber.Payee(payeeStripNonAlphanumeric(t.RemittanceInformationUnstructured)),
		Memo:    t.RemittanceInformationUnstructured,
		Amount:  ynabber.MilliunitsFromAmount(amount),
	}, nil
}
